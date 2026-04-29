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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient"
)

// RunnerReconciler reconciles a Runner object
type RunnerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=runners,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=runners/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=runners/finalizers,verbs=update
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=pools,verbs=get;list;watch

func (r *RunnerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := &garmv1alpha1.Runner{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if obj.Spec.PoolRef.Name == "" {
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
			Reason: garmv1alpha1.ReasonReferenceMiss, Message: "poolRef.name is required", ObservedGeneration: obj.Generation,
		})
		if err := r.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	pool := &garmv1alpha1.Pool{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.PoolRef.Name}, pool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.Delete(ctx, obj)
		}
		return ctrl.Result{}, err
	}
	if pool.Status.ID == "" {
		return r.markRunnerFalse(ctx, obj, fmt.Errorf("pool %s/%s not yet reconciled", obj.Namespace, pool.Name))
	}

	instances, err := r.Garm.ListPoolInstances(ctx, pool.Status.ID)
	if err != nil {
		return r.markRunnerFalse(ctx, obj, err)
	}
	for _, inst := range instances {
		if runnerMatchesInstance(obj, inst.ID, inst.Name) {
			updateRunnerStatus(obj, inst)
			if err := r.Status().Update(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
	}
	return ctrl.Result{}, r.Delete(ctx, obj)
}

func runnerMatchesInstance(runner *garmv1alpha1.Runner, id, name string) bool {
	return (runner.Status.ID != "" && runner.Status.ID == id) ||
		(runner.Status.Name != "" && runner.Status.Name == name) ||
		runner.Name == dnsLabel(runner.Spec.PoolRef.Name+"-"+name)
}

func (r *RunnerReconciler) markRunnerFalse(ctx context.Context, obj *garmv1alpha1.Runner, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: garmv1alpha1.ReasonAPIError, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func (r *RunnerReconciler) poolToRunners(ctx context.Context, pool client.Object) []reconcile.Request {
	list := &garmv1alpha1.RunnerList{}
	if err := r.List(ctx, list, client.InNamespace(pool.GetNamespace())); err != nil {
		return nil
	}
	var out []reconcile.Request
	for i := range list.Items {
		runner := &list.Items[i]
		if runner.Spec.PoolRef.Name == pool.GetName() {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: runner.Namespace, Name: runner.Name}})
		}
	}
	return out
}

// SetupWithManager sets up the controller with the Manager.
func (r *RunnerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.Runner{}).
		Watches(&garmv1alpha1.Pool{}, handler.EnqueueRequestsFromMapFunc(r.poolToRunners)).
		Named("runner").
		Complete(r)
}
