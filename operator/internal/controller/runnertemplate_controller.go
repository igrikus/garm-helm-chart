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
	"strconv"

	garmparams "github.com/cloudbase/garm/params"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient"
)

// RunnerTemplateReconciler reconciles a RunnerTemplate object.
type RunnerTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=runnertemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=runnertemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=runnertemplates/finalizers,verbs=update

func (r *RunnerTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &garmv1alpha1.RunnerTemplate{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.DeletionTimestamp.IsZero() {
		return r.deleteTemplate(ctx, obj)
	}

	if !controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
		controllerutil.AddFinalizer(obj, garmv1alpha1.Finalizer)
		return ctrl.Result{Requeue: true}, r.Update(ctx, obj)
	}

	desired := templateCreate(obj)
	if obj.Status.ID == "" {
		id, err := r.adoptOrCreateTemplate(ctx, desired)
		if err != nil {
			return r.markTemplateFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
		}
		obj.Status.ID = strconv.FormatUint(uint64(id), 10)
	} else {
		id, err := strconv.ParseUint(obj.Status.ID, 10, 64)
		if err != nil {
			obj.Status.ID = ""
			return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
		}
		actual, gerr := r.Garm.GetTemplate(ctx, uint(id))
		if garmclient.IsNotFound(gerr) {
			obj.Status.ID = ""
			return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
		}
		if gerr != nil {
			return r.markTemplateFalse(ctx, obj, garmv1alpha1.ReasonAPIError, gerr)
		}
		if string(actual.OSType) != desired.OSType || string(actual.ForgeType) != desired.ForgeType {
			if err := r.Garm.DeleteTemplate(ctx, actual.ID); err != nil && !garmclient.IsNotFound(err) {
				return r.markTemplateFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
			}
			newID, err := r.Garm.CreateTemplate(ctx, desired)
			if err != nil {
				return r.markTemplateFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
			}
			obj.Status.ID = strconv.FormatUint(uint64(newID), 10)
		} else if update := templateDiff(actual, desired); update != nil {
			if err := r.Garm.UpdateTemplate(ctx, actual.ID, *update); err != nil {
				return r.markTemplateFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
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

func (r *RunnerTemplateReconciler) adoptOrCreateTemplate(ctx context.Context, desired garmclient.TemplateCreate) (uint, error) {
	list, err := r.Garm.ListTemplates(ctx, desired.OSType, desired.ForgeType, desired.Name)
	if err != nil {
		return 0, err
	}
	var matches []garmparams.Template
	for _, t := range list {
		if t.Name == desired.Name && string(t.OSType) == desired.OSType && string(t.ForgeType) == desired.ForgeType {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 0:
		return r.Garm.CreateTemplate(ctx, desired)
	case 1:
		return matches[0].ID, nil
	default:
		return 0, fmt.Errorf("found %d existing templates named %q for %s/%s", len(matches), desired.Name, desired.ForgeType, desired.OSType)
	}
}

func (r *RunnerTemplateReconciler) deleteTemplate(ctx context.Context, obj *garmv1alpha1.RunnerTemplate) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
		return ctrl.Result{}, nil
	}
	if obj.Status.ID != "" {
		id, err := strconv.ParseUint(obj.Status.ID, 10, 64)
		if err == nil {
			if err := r.Garm.DeleteTemplate(ctx, uint(id)); err != nil && !garmclient.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
	}
	controllerutil.RemoveFinalizer(obj, garmv1alpha1.Finalizer)
	return ctrl.Result{}, r.Update(ctx, obj)
}

func (r *RunnerTemplateReconciler) markTemplateFalse(ctx context.Context, obj *garmv1alpha1.RunnerTemplate, reason string, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: reason, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func templateCreate(obj *garmv1alpha1.RunnerTemplate) garmclient.TemplateCreate {
	osType := string(obj.Spec.OSType)
	if osType == "" {
		osType = "linux"
	}
	return garmclient.TemplateCreate{
		Name:        obj.Name,
		Description: obj.Spec.Description,
		OSType:      osType,
		ForgeType:   string(obj.Spec.ForgeType),
		Data:        []byte(obj.Spec.Data),
	}
}

func templateDiff(actual *garmparams.Template, desired garmclient.TemplateCreate) *garmclient.TemplateUpdate {
	var out garmclient.TemplateUpdate
	changed := false
	if actual.Name != desired.Name {
		out.Name = &desired.Name
		changed = true
	}
	if actual.Description != desired.Description {
		out.Description = &desired.Description
		changed = true
	}
	if !bytes.Equal(actual.Data, desired.Data) {
		out.Data = desired.Data
		changed = true
	}
	if !changed {
		return nil
	}
	return &out
}

// SetupWithManager sets up the controller with the Manager.
func (r *RunnerTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.RunnerTemplate{}).
		Named("runnertemplate").
		Complete(r)
}
