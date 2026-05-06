/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient"
)

// GithubCredentialsReconciler reconciles a GithubCredentials object
type GithubCredentialsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=githubcredentials,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=githubcredentials/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=githubcredentials/finalizers,verbs=update
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=githubendpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *GithubCredentialsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	obj := &garmv1alpha1.GithubCredentials{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
			if id, ok := decodeCredID(obj.Status.ID); ok {
				if err := r.Garm.DeleteGithubCredentials(ctx, id); err != nil && !garmclient.IsNotFound(err) {
					log.Error(err, "delete github credentials")
					return ctrl.Result{}, err
				}
			}
			controllerutil.RemoveFinalizer(obj, garmv1alpha1.Finalizer)
			if err := r.Update(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
		controllerutil.AddFinalizer(obj, garmv1alpha1.Finalizer)
		return ctrl.Result{Requeue: true}, r.Update(ctx, obj)
	}

	endpoint := &garmv1alpha1.GithubEndpoint{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.EndpointRef.Name}, endpoint); err != nil {
		if apierrors.IsNotFound(err) {
			return r.markGithubCredsFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss,
				fmt.Errorf("endpoint %s/%s not found", obj.Namespace, obj.Spec.EndpointRef.Name))
		}
		return ctrl.Result{}, err
	}
	if endpoint.Status.ID == "" {
		return r.markGithubCredsFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss,
			fmt.Errorf("endpoint %s/%s not yet reconciled", obj.Namespace, endpoint.Name))
	}

	desired, err := r.buildGithubCredentialsSpec(ctx, obj, endpoint.Status.ID)
	if err != nil {
		return r.markGithubCredsFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}

	if id, ok := decodeCredID(obj.Status.ID); ok {
		if _, gerr := r.Garm.GetGithubCredentials(ctx, id); gerr != nil {
			if garmclient.IsNotFound(gerr) {
				obj.Status.ID = ""
				return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
			}
			return r.markGithubCredsFalse(ctx, obj, garmv1alpha1.ReasonAPIError, gerr)
		}
		desc := obj.Spec.Description
		upd := garmclient.GithubCredentialsUpdate{Description: &desc}
		if obj.Spec.AuthType == garmv1alpha1.AuthTypeApp {
			upd.AppID = desired.AppID
			upd.InstallationID = desired.InstallationID
			upd.PrivateKeyBytes = desired.PrivateKeyBytes
		} else {
			upd.OAuth2Token = &desired.OAuth2Token
		}
		if err := r.Garm.UpdateGithubCredentials(ctx, id, upd); err != nil {
			return r.markGithubCredsFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
		}
	} else {
		newID, err := r.Garm.CreateGithubCredentials(ctx, desired)
		if err != nil {
			return r.markGithubCredsFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
		}
		obj.Status.ID = strconv.FormatInt(newID, 10)
	}

	obj.Status.ObservedGeneration = obj.Generation
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionTrue,
		Reason: garmv1alpha1.ReasonReconciled, ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *GithubCredentialsReconciler) buildGithubCredentialsSpec(ctx context.Context, obj *garmv1alpha1.GithubCredentials, endpointID string) (garmclient.GithubCredentialsSpec, error) {
	out := garmclient.GithubCredentialsSpec{
		Name:        obj.Name,
		Description: obj.Spec.Description,
		Endpoint:    endpointID,
		AuthType:    string(obj.Spec.AuthType),
	}
	switch obj.Spec.AuthType {
	case garmv1alpha1.AuthTypePAT:
		if obj.Spec.PATSecretRef == nil {
			return out, fmt.Errorf("patSecretRef is required")
		}
		token, err := r.resolveGithubSecretString(ctx, obj.Namespace, *obj.Spec.PATSecretRef, "PAT")
		if err != nil {
			return out, err
		}
		out.OAuth2Token = token
	case garmv1alpha1.AuthTypeApp:
		if obj.Spec.AppAuth == nil {
			return out, fmt.Errorf("appAuth is required")
		}
		key, err := r.resolveGithubSecretBytes(ctx, obj.Namespace, obj.Spec.AppAuth.PrivateKeySecretRef, "private key")
		if err != nil {
			return out, err
		}
		out.AppID = obj.Spec.AppAuth.AppID
		out.InstallationID = obj.Spec.AppAuth.InstallationID
		out.PrivateKeyBytes = key
	default:
		return out, fmt.Errorf("unsupported authType %q", obj.Spec.AuthType)
	}
	return out, nil
}

func (r *GithubCredentialsReconciler) resolveGithubSecretString(ctx context.Context, namespace string, ref garmv1alpha1.SecretKeyRef, label string) (string, error) {
	v, err := r.resolveGithubSecretBytes(ctx, namespace, ref, label)
	if err != nil {
		return "", err
	}
	return string(v), nil
}

func (r *GithubCredentialsReconciler) resolveGithubSecretBytes(ctx context.Context, namespace string, ref garmv1alpha1.SecretKeyRef, label string) ([]byte, error) {
	sec := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, sec); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("%s secret %s/%s not found", label, namespace, ref.Name)
		}
		return nil, err
	}
	v, ok := sec.Data[ref.Key]
	if !ok || len(v) == 0 {
		return nil, fmt.Errorf("%s secret %s/%s missing key %q", label, namespace, ref.Name, ref.Key)
	}
	return v, nil
}

func (r *GithubCredentialsReconciler) markGithubCredsFalse(ctx context.Context, obj *garmv1alpha1.GithubCredentials, reason string, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: reason, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func (r *GithubCredentialsReconciler) endpointToGithubCreds(ctx context.Context, ep client.Object) []reconcile.Request {
	list := &garmv1alpha1.GithubCredentialsList{}
	if err := r.List(ctx, list, client.InNamespace(ep.GetNamespace())); err != nil {
		return nil
	}
	var out []reconcile.Request
	for i := range list.Items {
		c := &list.Items[i]
		if c.Spec.EndpointRef.Name == ep.GetName() {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: c.Namespace, Name: c.Name}})
		}
	}
	return out
}

// SetupWithManager sets up the controller with the Manager.
func (r *GithubCredentialsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.GithubCredentials{}).
		Watches(&garmv1alpha1.GithubEndpoint{}, handler.EnqueueRequestsFromMapFunc(r.endpointToGithubCreds)).
		Named("githubcredentials").
		Complete(r)
}
