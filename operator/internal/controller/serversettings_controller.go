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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	garmparams "github.com/cloudbase/garm/params"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient"
)

// ServerSettingsReconciler reconciles a ServerSettings object.
type ServerSettingsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=serversettings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=serversettings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ServerSettingsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &garmv1alpha1.ServerSettings{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	caBundle, err := r.resolveServerSettingsCABundle(ctx, obj)
	if err != nil {
		return r.markServerSettingsFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}

	actual, err := r.Garm.GetServerSettings(ctx)
	if err != nil {
		return r.markServerSettingsFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
	}

	update := serverSettingsDiff(actual, obj.Spec, caBundle)
	if update != nil {
		if err := r.Garm.UpdateServerSettings(ctx, *update); err != nil {
			return r.markServerSettingsFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
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

func (r *ServerSettingsReconciler) resolveServerSettingsCABundle(ctx context.Context, obj *garmv1alpha1.ServerSettings) ([]byte, error) {
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

func serverSettingsDiff(actual *garmparams.ControllerInfo, desired garmv1alpha1.ServerSettingsSpec, caBundle []byte) *garmclient.ServerSettingsUpdate {
	var update garmclient.ServerSettingsUpdate
	changed := false

	if desired.MetadataURL != nil && actual.MetadataURL != *desired.MetadataURL {
		update.MetadataURL = desired.MetadataURL
		changed = true
	}
	if desired.CallbackURL != nil && actual.CallbackURL != *desired.CallbackURL {
		update.CallbackURL = desired.CallbackURL
		changed = true
	}
	if desired.WebhookURL != nil && actual.WebhookURL != *desired.WebhookURL {
		update.WebhookURL = desired.WebhookURL
		changed = true
	}
	if desired.AgentURL != nil && actual.AgentURL != *desired.AgentURL {
		update.AgentURL = desired.AgentURL
		changed = true
	}
	if desired.GARMAgentReleasesURL != nil && actual.GARMAgentReleasesURL != *desired.GARMAgentReleasesURL {
		update.GARMAgentReleasesURL = desired.GARMAgentReleasesURL
		changed = true
	}
	if desired.SyncGARMAgentTools != nil && actual.SyncGARMAgentTools != *desired.SyncGARMAgentTools {
		update.SyncGARMAgentTools = desired.SyncGARMAgentTools
		changed = true
	}
	if desired.MinimumJobAgeBackoffSeconds != nil && actual.MinimumJobAgeBackoff != *desired.MinimumJobAgeBackoffSeconds {
		update.MinimumJobAgeBackoffSeconds = desired.MinimumJobAgeBackoffSeconds
		changed = true
	}
	if desired.CACertBundleSecretRef == nil {
		if len(actual.CACertBundle) > 0 {
			update.ClearCACertBundle = true
			changed = true
		}
	} else if !bytes.Equal(actual.CACertBundle, caBundle) {
		if len(caBundle) == 0 {
			update.ClearCACertBundle = true
		} else {
			update.CACertBundle = caBundle
		}
		changed = true
	}

	if !changed {
		return nil
	}
	return &update
}

func (r *ServerSettingsReconciler) markServerSettingsFalse(ctx context.Context, obj *garmv1alpha1.ServerSettings, reason string, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: reason, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func (r *ServerSettingsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.ServerSettings{}).
		Named("serversettings").
		Complete(r)
}
