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
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	garmparams "github.com/cloudbase/garm/params"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient"
)

// GithubEndpointReconciler reconciles a GithubEndpoint object
type GithubEndpointReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=githubendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=githubendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=githubendpoints/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *GithubEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	obj := &garmv1alpha1.GithubEndpoint{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
			if obj.Status.ID != "" {
				if err := r.Garm.DeleteGithubEndpoint(ctx, obj.Status.ID); err != nil && !garmclient.IsNotFound(err) {
					log.Error(err, "delete github endpoint")
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

	caBundle, err := r.resolveGithubCABundle(ctx, obj)
	if err != nil {
		return r.markGithubEndpointFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}
	desired, err := buildGithubEndpointSpec(obj, caBundle)
	if err != nil {
		return r.markGithubEndpointFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}

	if obj.Status.ID == "" {
		name, err := r.Garm.CreateGithubEndpoint(ctx, desired)
		if err != nil {
			if !garmclient.IsConflict(err) {
				return r.markGithubEndpointFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
			}
			actual, getErr := r.Garm.GetGithubEndpoint(ctx, desired.Name)
			if getErr != nil {
				return r.markGithubEndpointFalse(ctx, obj, garmv1alpha1.ReasonAPIError, getErr)
			}
			obj.Status.ID = actual.Name
			if githubEndpointDrifted(actual, desired) {
				if err := r.Garm.UpdateGithubEndpoint(ctx, obj.Status.ID, desired); err != nil {
					return r.markGithubEndpointFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
				}
			}
		} else {
			obj.Status.ID = name
		}
	} else {
		actual, err := r.Garm.GetGithubEndpoint(ctx, obj.Status.ID)
		if garmclient.IsNotFound(err) {
			obj.Status.ID = ""
			return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
		}
		if err != nil {
			return r.markGithubEndpointFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
		}
		if githubEndpointDrifted(actual, desired) {
			if err := r.Garm.UpdateGithubEndpoint(ctx, obj.Status.ID, desired); err != nil {
				return r.markGithubEndpointFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
			}
		}
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

func buildGithubEndpointSpec(obj *garmv1alpha1.GithubEndpoint, caBundle []byte) (garmclient.GithubEndpointSpec, error) {
	out := garmclient.GithubEndpointSpec{
		Name:          obj.Name,
		Description:   obj.Spec.Description,
		BaseURL:       obj.Spec.BaseURL,
		APIBaseURL:    obj.Spec.APIBaseURL,
		UploadBaseURL: obj.Spec.UploadBaseURL,
		CACertBundle:  caBundle,
	}
	if out.BaseURL == "https://github.com" {
		if out.APIBaseURL == "" {
			out.APIBaseURL = "https://api.github.com"
		}
		if out.UploadBaseURL == "" {
			out.UploadBaseURL = "https://uploads.github.com"
		}
	}
	if out.APIBaseURL == "" || out.UploadBaseURL == "" {
		return out, fmt.Errorf("apiBaseURL and uploadBaseURL are required for GitHub Enterprise endpoint %q", out.BaseURL)
	}
	return out, nil
}

func (r *GithubEndpointReconciler) resolveGithubCABundle(ctx context.Context, obj *garmv1alpha1.GithubEndpoint) ([]byte, error) {
	if obj.Spec.CACertBundleSecretRef == nil {
		return nil, nil
	}
	ref := obj.Spec.CACertBundleSecretRef
	sec := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: ref.Name}, sec); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("CA bundle secret %s/%s not found", obj.Namespace, ref.Name)
		}
		return nil, err
	}
	v, ok := sec.Data[ref.Key]
	if !ok {
		return nil, fmt.Errorf("CA bundle secret %s/%s missing key %q", obj.Namespace, ref.Name, ref.Key)
	}
	return v, nil
}

func (r *GithubEndpointReconciler) markGithubEndpointFalse(ctx context.Context, obj *garmv1alpha1.GithubEndpoint, reason string, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: reason, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func githubEndpointDrifted(actual *garmparams.ForgeEndpoint, desired garmclient.GithubEndpointSpec) bool {
	return actual.APIBaseURL != desired.APIBaseURL ||
		actual.UploadBaseURL != desired.UploadBaseURL ||
		actual.BaseURL != desired.BaseURL ||
		actual.Description != desired.Description ||
		!bytes.Equal(actual.CACertBundle, desired.CACertBundle)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GithubEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.GithubEndpoint{}).
		Named("githubendpoint").
		Complete(r)
}
