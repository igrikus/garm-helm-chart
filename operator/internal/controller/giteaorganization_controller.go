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

	garmparams "github.com/cloudbase/garm/params"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient"
)

// GiteaOrganizationReconciler reconciles a GiteaOrganization object
type GiteaOrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteaorganizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteaorganizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteaorganizations/finalizers,verbs=update
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteacredentials,verbs=get;list;watch

func (r *GiteaOrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	obj := &garmv1alpha1.GiteaOrganization{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
			if obj.Status.ID != "" {
				if err := r.Garm.DeleteOrg(ctx, obj.Status.ID); err != nil && !garmclient.IsNotFound(err) {
					log.Error(err, "delete gitea organization")
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

	creds := &garmv1alpha1.GiteaCredentials{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.CredentialsRef.Name}, creds); err != nil {
		if apierrors.IsNotFound(err) {
			return r.markOrgFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss,
				fmt.Errorf("credentials %s/%s not found", obj.Namespace, obj.Spec.CredentialsRef.Name))
		}
		return ctrl.Result{}, err
	}
	if creds.Status.ID == "" {
		return r.markOrgFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss,
			fmt.Errorf("credentials %s/%s not yet reconciled", obj.Namespace, creds.Name))
	}

	webhookSecret, err := r.resolveWebhookSecret(ctx, obj)
	if err != nil {
		return r.markOrgFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}

	balancer := string(obj.Spec.PoolBalancerType)
	if balancer == "" {
		balancer = string(garmv1alpha1.PoolBalancerRoundRobin)
	}

	if obj.Status.ID == "" {
		id, err := r.Garm.CreateOrg(ctx, garmclient.OrgSpec{
			Name:             obj.Spec.Name,
			CredentialsName:  creds.Name,
			WebhookSecret:    webhookSecret,
			PoolBalancerType: balancer,
			ForgeType:        string(garmparams.GiteaEndpointType),
		})
		if err != nil {
			return r.markOrgFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
		}
		obj.Status.ID = id
	} else {
		actual, gerr := r.Garm.GetOrg(ctx, obj.Status.ID)
		if garmclient.IsNotFound(gerr) {
			obj.Status.ID = ""
			return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
		}
		if gerr != nil {
			return r.markOrgFalse(ctx, obj, garmv1alpha1.ReasonAPIError, gerr)
		}
		upd := garmclient.OrgUpdate{}
		if actual.CredentialsName != creds.Name {
			n := creds.Name
			upd.CredentialsName = &n
		}
		if string(actual.PoolBalancerType) != balancer {
			b := balancer
			upd.PoolBalancerType = &b
		}
		if upd.CredentialsName != nil || upd.PoolBalancerType != nil {
			if err := r.Garm.UpdateOrg(ctx, obj.Status.ID, upd); err != nil {
				return r.markOrgFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
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

func (r *GiteaOrganizationReconciler) resolveWebhookSecret(ctx context.Context, obj *garmv1alpha1.GiteaOrganization) (string, error) {
	if obj.Spec.WebhookSecretRef == nil {
		return "", nil
	}
	ref := obj.Spec.WebhookSecretRef
	sec := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: ref.Name}, sec); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("webhook secret %s/%s not found", obj.Namespace, ref.Name)
		}
		return "", err
	}
	v, ok := sec.Data[ref.Key]
	if !ok || len(v) == 0 {
		return "", fmt.Errorf("webhook secret %s/%s missing key %q", obj.Namespace, ref.Name, ref.Key)
	}
	return string(v), nil
}

func (r *GiteaOrganizationReconciler) markOrgFalse(ctx context.Context, obj *garmv1alpha1.GiteaOrganization, reason string, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: reason, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func (r *GiteaOrganizationReconciler) credsToOrgs(ctx context.Context, c client.Object) []reconcile.Request {
	list := &garmv1alpha1.GiteaOrganizationList{}
	if err := r.List(ctx, list, client.InNamespace(c.GetNamespace())); err != nil {
		return nil
	}
	var out []reconcile.Request
	for i := range list.Items {
		o := &list.Items[i]
		if o.Spec.CredentialsRef.Name == c.GetName() {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: o.Namespace, Name: o.Name}})
		}
	}
	return out
}

func (r *GiteaOrganizationReconciler) endpointToOrgs(ctx context.Context, ep client.Object) []reconcile.Request {
	list := &garmv1alpha1.GiteaOrganizationList{}
	if err := r.List(ctx, list, client.InNamespace(ep.GetNamespace())); err != nil {
		return nil
	}
	var out []reconcile.Request
	for i := range list.Items {
		o := &list.Items[i]
		if o.Spec.EndpointRef.Name == ep.GetName() {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: o.Namespace, Name: o.Name}})
		}
	}
	return out
}

func (r *GiteaOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.GiteaOrganization{}).
		Watches(&garmv1alpha1.GiteaCredentials{}, handler.EnqueueRequestsFromMapFunc(r.credsToOrgs)).
		Watches(&garmv1alpha1.GiteaEndpoint{}, handler.EnqueueRequestsFromMapFunc(r.endpointToOrgs)).
		Named("giteaorganization").
		Complete(r)
}
