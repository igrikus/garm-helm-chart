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

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=repositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=repositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=repositories/finalizers,verbs=update
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteaendpoints;githubendpoints;giteacredentials;githubcredentials,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *RepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	obj := &garmv1alpha1.Repository{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
			if obj.Status.ID != "" {
				if err := r.Garm.DeleteRepo(ctx, obj.Status.ID, false); err != nil && !garmclient.IsNotFound(err) {
					log.Error(err, "delete repository")
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

	credsName, forgeType, err := r.resolveRepoRefs(ctx, obj)
	if err != nil {
		return r.markRepoFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}
	webhookSecret, err := r.resolveRepoWebhookSecret(ctx, obj)
	if err != nil {
		return r.markRepoFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}
	balancer := string(obj.Spec.PoolBalancerType)
	if balancer == "" {
		balancer = string(garmv1alpha1.PoolBalancerRoundRobin)
	}

	if obj.Status.ID == "" {
		id, err := r.Garm.CreateRepo(ctx, garmclient.RepoSpec{
			Owner: obj.Spec.Owner, Name: obj.Spec.Name, CredentialsName: credsName,
			WebhookSecret: webhookSecret, PoolBalancerType: balancer, ForgeType: forgeType,
		})
		if err != nil {
			return r.markRepoFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
		}
		obj.Status.ID = id
	} else {
		actual, gerr := r.Garm.GetRepo(ctx, obj.Status.ID)
		if garmclient.IsNotFound(gerr) {
			obj.Status.ID = ""
			return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
		}
		if gerr != nil {
			return r.markRepoFalse(ctx, obj, garmv1alpha1.ReasonAPIError, gerr)
		}
		if actual.Owner != obj.Spec.Owner || actual.Name != obj.Spec.Name {
			if err := r.Garm.DeleteRepo(ctx, obj.Status.ID, false); err != nil && !garmclient.IsNotFound(err) {
				return r.markRepoFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
			}
			obj.Status.ID = ""
			return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
		}
		upd := garmclient.EntityUpdate{}
		if actual.GetCredentialsName() != credsName {
			n := credsName
			upd.CredentialsName = &n
		}
		if string(actual.GetBalancerType()) != balancer {
			b := balancer
			upd.PoolBalancerType = &b
		}
		if obj.Spec.WebhookSecretRef != nil {
			secret := webhookSecret
			upd.WebhookSecret = &secret
		}
		if upd.CredentialsName != nil || upd.PoolBalancerType != nil || upd.WebhookSecret != nil {
			if err := r.Garm.UpdateRepo(ctx, obj.Status.ID, upd); err != nil {
				return r.markRepoFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
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

func (r *RepositoryReconciler) resolveRepoRefs(ctx context.Context, obj *garmv1alpha1.Repository) (string, string, error) {
	switch obj.Spec.ForgeRef.Kind {
	case "GiteaEndpoint":
		ep := &garmv1alpha1.GiteaEndpoint{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.ForgeRef.Name}, ep); err != nil {
			if apierrors.IsNotFound(err) {
				return "", "", fmt.Errorf("gitea endpoint %s/%s not found", obj.Namespace, obj.Spec.ForgeRef.Name)
			}
			return "", "", err
		}
		if ep.Status.ID == "" {
			return "", "", fmt.Errorf("gitea endpoint %s/%s not yet reconciled", obj.Namespace, ep.Name)
		}
		creds := &garmv1alpha1.GiteaCredentials{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.CredentialsRef.Name}, creds); err != nil {
			if apierrors.IsNotFound(err) {
				return "", "", fmt.Errorf("gitea credentials %s/%s not found", obj.Namespace, obj.Spec.CredentialsRef.Name)
			}
			return "", "", err
		}
		if creds.Spec.EndpointRef.Name != obj.Spec.ForgeRef.Name {
			return "", "", fmt.Errorf("credentials %s/%s reference endpoint %q, want %q", obj.Namespace, creds.Name, creds.Spec.EndpointRef.Name, obj.Spec.ForgeRef.Name)
		}
		if creds.Status.ID == "" {
			return "", "", fmt.Errorf("gitea credentials %s/%s not yet reconciled", obj.Namespace, creds.Name)
		}
		return creds.Name, string(garmparams.GiteaEndpointType), nil
	case "GithubEndpoint":
		ep := &garmv1alpha1.GithubEndpoint{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.ForgeRef.Name}, ep); err != nil {
			if apierrors.IsNotFound(err) {
				return "", "", fmt.Errorf("github endpoint %s/%s not found", obj.Namespace, obj.Spec.ForgeRef.Name)
			}
			return "", "", err
		}
		if ep.Status.ID == "" {
			return "", "", fmt.Errorf("github endpoint %s/%s not yet reconciled", obj.Namespace, ep.Name)
		}
		creds := &garmv1alpha1.GithubCredentials{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.CredentialsRef.Name}, creds); err != nil {
			if apierrors.IsNotFound(err) {
				return "", "", fmt.Errorf("github credentials %s/%s not found", obj.Namespace, obj.Spec.CredentialsRef.Name)
			}
			return "", "", err
		}
		if creds.Spec.EndpointRef.Name != obj.Spec.ForgeRef.Name {
			return "", "", fmt.Errorf("credentials %s/%s reference endpoint %q, want %q", obj.Namespace, creds.Name, creds.Spec.EndpointRef.Name, obj.Spec.ForgeRef.Name)
		}
		if creds.Status.ID == "" {
			return "", "", fmt.Errorf("github credentials %s/%s not yet reconciled", obj.Namespace, creds.Name)
		}
		return creds.Name, string(garmparams.GithubEndpointType), nil
	default:
		return "", "", fmt.Errorf("unsupported forgeRef.kind %q", obj.Spec.ForgeRef.Kind)
	}
}

func (r *RepositoryReconciler) resolveRepoWebhookSecret(ctx context.Context, obj *garmv1alpha1.Repository) (string, error) {
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

func (r *RepositoryReconciler) markRepoFalse(ctx context.Context, obj *garmv1alpha1.Repository, reason string, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: reason, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func (r *RepositoryReconciler) refToRepos(ctx context.Context, ref client.Object) []reconcile.Request {
	list := &garmv1alpha1.RepositoryList{}
	if err := r.List(ctx, list, client.InNamespace(ref.GetNamespace())); err != nil {
		return nil
	}
	var out []reconcile.Request
	for i := range list.Items {
		repo := &list.Items[i]
		if repo.Spec.CredentialsRef.Name == ref.GetName() || repo.Spec.ForgeRef.Name == ref.GetName() {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: repo.Namespace, Name: repo.Name}})
		}
	}
	return out
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.Repository{}).
		Watches(&garmv1alpha1.GiteaEndpoint{}, handler.EnqueueRequestsFromMapFunc(r.refToRepos)).
		Watches(&garmv1alpha1.GithubEndpoint{}, handler.EnqueueRequestsFromMapFunc(r.refToRepos)).
		Watches(&garmv1alpha1.GiteaCredentials{}, handler.EnqueueRequestsFromMapFunc(r.refToRepos)).
		Watches(&garmv1alpha1.GithubCredentials{}, handler.EnqueueRequestsFromMapFunc(r.refToRepos)).
		Named("repository").
		Complete(r)
}
