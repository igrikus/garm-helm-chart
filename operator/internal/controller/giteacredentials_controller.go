/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
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

// GiteaCredentialsReconciler reconciles a GiteaCredentials object
type GiteaCredentialsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteacredentials,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteacredentials/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteacredentials/finalizers,verbs=update
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteaendpoints,verbs=get;list;watch

func (r *GiteaCredentialsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	obj := &garmv1alpha1.GiteaCredentials{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
			if id, ok := decodeCredID(obj.Status.ID); ok {
				if err := r.Garm.DeleteGiteaCredentials(ctx, id); err != nil && !garmclient.IsNotFound(err) {
					log.Error(err, "delete gitea credentials")
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

	endpoint := &garmv1alpha1.GiteaEndpoint{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.EndpointRef.Name}, endpoint); err != nil {
		if apierrors.IsNotFound(err) {
			return r.markFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss,
				fmt.Errorf("endpoint %s/%s not found", obj.Namespace, obj.Spec.EndpointRef.Name))
		}
		return ctrl.Result{}, err
	}
	if endpoint.Status.ID == "" {
		return r.markFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss,
			fmt.Errorf("endpoint %s/%s not yet reconciled", obj.Namespace, endpoint.Name))
	}

	pat, err := r.resolvePAT(ctx, obj)
	if err != nil {
		return r.markFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}

	if id, ok := decodeCredID(obj.Status.ID); ok {
		if _, gerr := r.Garm.GetGiteaCredentials(ctx, id); gerr != nil {
			if garmclient.IsNotFound(gerr) {
				obj.Status.ID = ""
				return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
			}
			return r.markFalse(ctx, obj, garmv1alpha1.ReasonAPIError, gerr)
		}
		// PAT is write-only, so we cannot diff it. Always rotate to keep the cluster
		// as the source of truth — cheap on GARM side and matches "edit Secret → propagate".
		desc := obj.Spec.Description
		oauth := pat
		if err := r.Garm.UpdateGiteaCredentials(ctx, id, garmclient.GiteaCredentialsUpdate{
			Description: &desc, OAuth2Token: &oauth,
		}); err != nil {
			return r.markFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
		}
	} else {
		newID, err := r.Garm.CreateGiteaCredentials(ctx, garmclient.GiteaCredentialsSpec{
			Name:        obj.Name,
			Description: obj.Spec.Description,
			Endpoint:    endpoint.Status.ID,
			OAuth2Token: pat,
		})
		if err != nil {
			return r.markFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
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

func (r *GiteaCredentialsReconciler) resolvePAT(ctx context.Context, obj *garmv1alpha1.GiteaCredentials) (string, error) {
	ref := obj.Spec.PATSecretRef
	sec := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: ref.Name}, sec); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("PAT secret %s/%s not found", obj.Namespace, ref.Name)
		}
		return "", err
	}
	v, ok := sec.Data[ref.Key]
	if !ok || len(v) == 0 {
		return "", fmt.Errorf("PAT secret %s/%s missing key %q", obj.Namespace, ref.Name, ref.Key)
	}
	return string(v), nil
}

func (r *GiteaCredentialsReconciler) markFalse(ctx context.Context, obj *garmv1alpha1.GiteaCredentials, reason string, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: reason, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func decodeCredID(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func (r *GiteaCredentialsReconciler) endpointToCreds(ctx context.Context, ep client.Object) []reconcile.Request {
	list := &garmv1alpha1.GiteaCredentialsList{}
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

func (r *GiteaCredentialsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.GiteaCredentials{}).
		Watches(&garmv1alpha1.GiteaEndpoint{}, handler.EnqueueRequestsFromMapFunc(r.endpointToCreds)).
		Named("giteacredentials").
		Complete(r)
}
