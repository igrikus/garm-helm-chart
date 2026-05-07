/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"bytes"
	"context"
	"fmt"
	"time"

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
	"github.com/cloudbase/garm/util/appdefaults"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient"
)

// GiteaEndpointReconciler reconciles a GiteaEndpoint object
type GiteaEndpointReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// requeueAfter is the periodic drift-correction interval. Spec changes
// re-enqueue immediately via the watch; this just covers out-of-band edits
// to GARM (someone running `garm-cli endpoint update` by hand).
const requeueAfter = 60 * time.Second

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteaendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteaendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteaendpoints/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *GiteaEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	obj := &garmv1alpha1.GiteaEndpoint{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Deletion path.
	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
			if obj.Status.ID != "" {
				if err := r.Garm.DeleteGiteaEndpoint(ctx, obj.Status.ID); err != nil && !garmclient.IsNotFound(err) {
					log.Error(err, "delete gitea endpoint")
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

	// Ensure finalizer present before any GARM-side state is created.
	if !controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
		controllerutil.AddFinalizer(obj, garmv1alpha1.Finalizer)
		return ctrl.Result{Requeue: true}, r.Update(ctx, obj)
	}

	// Resolve CA bundle once; same Secret read for create and update.
	caBundle, err := r.resolveCABundle(ctx, obj)
	if err != nil {
		return r.markSyncedFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}

	desired := garmclient.GiteaEndpointSpec{
		Name:                     obj.Name,
		Description:              obj.Spec.Description,
		APIBaseURL:               obj.Spec.APIBaseURL,
		BaseURL:                  obj.Spec.BaseURL,
		CACertBundle:             caBundle,
		ToolsMetadataURL:         obj.Spec.ToolsMetadataURL,
		UseInternalToolsMetadata: obj.Spec.UseInternalToolsMetadata,
	}
	if desired.APIBaseURL == "" {
		desired.APIBaseURL = desired.BaseURL
	}
	if desired.ToolsMetadataURL == "" {
		desired.ToolsMetadataURL = appdefaults.GiteaRunnerReleasesURL
	}

	if obj.Status.ID == "" {
		name, err := r.Garm.CreateGiteaEndpoint(ctx, desired)
		if err != nil {
			if !garmclient.IsConflict(err) {
				return r.markSyncedFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
			}
			actual, getErr := r.Garm.GetGiteaEndpoint(ctx, desired.Name)
			if getErr != nil {
				return r.markSyncedFalse(ctx, obj, garmv1alpha1.ReasonAPIError, getErr)
			}
			obj.Status.ID = actual.Name
			if endpointDrifted(actual, desired) {
				if err := r.Garm.UpdateGiteaEndpoint(ctx, obj.Status.ID, desired); err != nil {
					return r.markSyncedFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
				}
			}
		} else {
			obj.Status.ID = name
		}
	} else {
		actual, err := r.Garm.GetGiteaEndpoint(ctx, obj.Status.ID)
		if garmclient.IsNotFound(err) {
			// GARM lost it. Clear status; next reconcile recreates.
			obj.Status.ID = ""
			return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
		}
		if err != nil {
			return r.markSyncedFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
		}
		if endpointDrifted(actual, desired) {
			if err := r.Garm.UpdateGiteaEndpoint(ctx, obj.Status.ID, desired); err != nil {
				return r.markSyncedFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
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

func (r *GiteaEndpointReconciler) resolveCABundle(ctx context.Context, obj *garmv1alpha1.GiteaEndpoint) ([]byte, error) {
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

func (r *GiteaEndpointReconciler) markSyncedFalse(ctx context.Context, obj *garmv1alpha1.GiteaEndpoint, reason string, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: reason, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func endpointDrifted(actual *garmparams.ForgeEndpoint, desired garmclient.GiteaEndpointSpec) bool {
	useInternalToolsMetadata := false
	if actual.UseInternalToolsMetadata != nil {
		useInternalToolsMetadata = *actual.UseInternalToolsMetadata
	}
	return actual.APIBaseURL != desired.APIBaseURL ||
		actual.BaseURL != desired.BaseURL ||
		actual.Description != desired.Description ||
		actual.ToolsMetadataURL != desired.ToolsMetadataURL ||
		useInternalToolsMetadata != desired.UseInternalToolsMetadata ||
		!bytes.Equal(actual.CACertBundle, desired.CACertBundle)
}

func (r *GiteaEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.GiteaEndpoint{}).
		Named("giteaendpoint").
		Complete(r)
}
