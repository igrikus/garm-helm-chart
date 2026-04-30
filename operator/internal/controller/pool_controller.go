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
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

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

const (
	drainDisabledRequeue = 15 * time.Second
	drainWaitRequeue     = 30 * time.Second
)

// PoolReconciler reconciles a Pool object
type PoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Garm   garmclient.Interface
}

// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=pools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=pools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=pools/finalizers,verbs=update
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=runnertemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=giteaorganizations,verbs=get;list;watch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=repositories;enterprises,verbs=get;list;watch
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=runners,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=garm.igrikus.dev,resources=runners/status,verbs=get;update;patch

func (r *PoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	obj := &garmv1alpha1.Pool{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.DeletionTimestamp.IsZero() {
		return r.drain(ctx, obj)
	}

	if !controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
		controllerutil.AddFinalizer(obj, garmv1alpha1.Finalizer)
		return ctrl.Result{Requeue: true}, r.Update(ctx, obj)
	}

	scopeID, err := r.resolvePoolScope(ctx, obj)
	if err != nil {
		return r.markPoolFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}

	templateID, err := r.resolveRunnerTemplate(ctx, obj)
	if err != nil {
		return r.markPoolFalse(ctx, obj, garmv1alpha1.ReasonReferenceMiss, err)
	}

	desired := buildPoolCreate(&obj.Spec, obj.Spec.Image, templateID)

	if obj.Status.ID == "" {
		id, err := r.adoptOrCreatePool(ctx, obj.Spec.ScopeRef.Kind, scopeID, desired)
		if err != nil {
			return r.markPoolFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
		}
		obj.Status.ID = id
	} else {
		actual, gerr := r.Garm.GetPool(ctx, obj.Status.ID)
		if garmclient.IsNotFound(gerr) {
			obj.Status.ID = ""
			return ctrl.Result{Requeue: true}, r.Status().Update(ctx, obj)
		}
		if gerr != nil {
			return r.markPoolFalse(ctx, obj, garmv1alpha1.ReasonAPIError, gerr)
		}
		diff := poolDiff(actual, desired)
		if !diff.IsEmpty() {
			if err := r.Garm.UpdatePool(ctx, obj.Status.ID, diff); err != nil {
				return r.markPoolFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
			}
		}
	}

	instances, err := r.Garm.ListPoolInstances(ctx, obj.Status.ID)
	if err != nil {
		log.Error(err, "list pool instances")
	} else {
		var idle uint32
		for _, i := range instances {
			if i.RunnerStatus == garmparams.RunnerIdle {
				idle++
			}
		}
		obj.Status.IdleRunners = idle
		if err := r.syncRunnerMirrors(ctx, obj, instances); err != nil {
			return r.markPoolFalse(ctx, obj, garmv1alpha1.ReasonAPIError, err)
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

func (r *PoolReconciler) resolvePoolScope(ctx context.Context, obj *garmv1alpha1.Pool) (string, error) {
	switch obj.Spec.ScopeRef.Kind {
	case "GiteaOrganization":
		org := &garmv1alpha1.GiteaOrganization{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.ScopeRef.Name}, org); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("organization %s/%s not found", obj.Namespace, obj.Spec.ScopeRef.Name)
			}
			return "", err
		}
		if org.Status.ID == "" {
			return "", fmt.Errorf("organization %s/%s not yet reconciled", obj.Namespace, org.Name)
		}
		return org.Status.ID, nil
	case "Repository":
		repo := &garmv1alpha1.Repository{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.ScopeRef.Name}, repo); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("repository %s/%s not found", obj.Namespace, obj.Spec.ScopeRef.Name)
			}
			return "", err
		}
		if repo.Status.ID == "" {
			return "", fmt.Errorf("repository %s/%s not yet reconciled", obj.Namespace, repo.Name)
		}
		return repo.Status.ID, nil
	case "Enterprise":
		ent := &garmv1alpha1.Enterprise{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: obj.Spec.ScopeRef.Name}, ent); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("enterprise %s/%s not found", obj.Namespace, obj.Spec.ScopeRef.Name)
			}
			return "", err
		}
		if ent.Status.ID == "" {
			return "", fmt.Errorf("enterprise %s/%s not yet reconciled", obj.Namespace, ent.Name)
		}
		return ent.Status.ID, nil
	default:
		return "", fmt.Errorf("scope kind %q not supported", obj.Spec.ScopeRef.Kind)
	}
}

func (r *PoolReconciler) resolveRunnerTemplate(ctx context.Context, obj *garmv1alpha1.Pool) (uint, error) {
	if obj.Spec.RunnerInstallTemplateRef == nil || obj.Spec.RunnerInstallTemplateRef.Name == "" {
		return 0, nil
	}
	name := obj.Spec.RunnerInstallTemplateRef.Name
	tpl := &garmv1alpha1.RunnerTemplate{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: name}, tpl); err != nil {
		if apierrors.IsNotFound(err) {
			return 0, fmt.Errorf("runner template %s/%s not found", obj.Namespace, name)
		}
		return 0, err
	}
	if tpl.Status.ID == "" {
		return 0, fmt.Errorf("runner template %s/%s not yet reconciled", obj.Namespace, name)
	}
	ready := meta.FindStatusCondition(tpl.Status.Conditions, garmv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue {
		return 0, fmt.Errorf("runner template %s/%s is not ready", obj.Namespace, name)
	}
	id, err := strconv.ParseUint(tpl.Status.ID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("runner template %s/%s has invalid status id %q", obj.Namespace, name, tpl.Status.ID)
	}
	return uint(id), nil
}

func (r *PoolReconciler) adoptOrCreatePool(ctx context.Context, scopeKind, scopeID string, desired garmclient.PoolCreate) (string, error) {
	var (
		pools []garmparams.Pool
		err   error
	)
	switch scopeKind {
	case "GiteaOrganization":
		pools, err = r.Garm.ListOrgPools(ctx, scopeID)
	case "Repository":
		pools, err = r.Garm.ListRepoPools(ctx, scopeID)
	case "Enterprise":
		pools, err = r.Garm.ListEnterprisePools(ctx, scopeID)
	default:
		return "", fmt.Errorf("scope kind %q not supported", scopeKind)
	}
	if err != nil {
		return "", err
	}
	var matches []garmparams.Pool
	for _, p := range pools {
		if p.RunnerPrefix.Prefix == desired.RunnerPrefix {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		switch scopeKind {
		case "GiteaOrganization":
			return r.Garm.CreateOrgPool(ctx, scopeID, desired)
		case "Repository":
			return r.Garm.CreateRepoPool(ctx, scopeID, desired)
		case "Enterprise":
			return r.Garm.CreateEnterprisePool(ctx, scopeID, desired)
		}
		return "", fmt.Errorf("scope kind %q not supported", scopeKind)
	case 1:
		return matches[0].ID, nil
	default:
		return "", fmt.Errorf("found %d existing %s pools with runner prefix %q", len(matches), scopeKind, desired.RunnerPrefix)
	}
}

func (r *PoolReconciler) drain(ctx context.Context, obj *garmv1alpha1.Pool) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(obj, garmv1alpha1.Finalizer) {
		return ctrl.Result{}, nil
	}
	if obj.Status.ID == "" {
		controllerutil.RemoveFinalizer(obj, garmv1alpha1.Finalizer)
		return ctrl.Result{}, r.Update(ctx, obj)
	}

	actual, err := r.Garm.GetPool(ctx, obj.Status.ID)
	if garmclient.IsNotFound(err) {
		controllerutil.RemoveFinalizer(obj, garmv1alpha1.Finalizer)
		return ctrl.Result{}, r.Update(ctx, obj)
	}
	if err != nil {
		log.Error(err, "drain: get pool")
		return ctrl.Result{}, err
	}

	if actual.Enabled {
		f := false
		if err := r.Garm.UpdatePool(ctx, obj.Status.ID, garmclient.PoolUpdate{Enabled: &f}); err != nil {
			return ctrl.Result{}, err
		}
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type: garmv1alpha1.ConditionDraining, Status: metav1.ConditionTrue,
			Reason: "Disabled", Message: "pool disabled, awaiting drain", ObservedGeneration: obj.Generation,
		})
		if err := r.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: drainDisabledRequeue}, nil
	}

	instances, err := r.Garm.ListPoolInstances(ctx, obj.Status.ID)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, i := range instances {
		if isDrainableRunner(i.RunnerStatus) {
			if err := r.Garm.DeleteInstance(ctx, i.Name, false); err != nil && !garmclient.IsNotFound(err) {
				log.Error(err, "drain: delete instance", "name", i.Name)
			}
		}
	}
	if len(instances) > 0 {
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type: garmv1alpha1.ConditionDraining, Status: metav1.ConditionTrue,
			Reason:             "WaitingForRunners",
			Message:            fmt.Sprintf("%d runner(s) still present", len(instances)),
			ObservedGeneration: obj.Generation,
		})
		if err := r.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: drainWaitRequeue}, nil
	}

	if err := r.Garm.DeletePool(ctx, obj.Status.ID); err != nil && !garmclient.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(obj, garmv1alpha1.Finalizer)
	return ctrl.Result{}, r.Update(ctx, obj)
}

func isDrainableRunner(s garmparams.RunnerStatus) bool {
	switch s {
	case garmparams.RunnerIdle, garmparams.RunnerPending, garmparams.RunnerFailed, garmparams.RunnerTerminated:
		return true
	}
	return false
}

func (r *PoolReconciler) markPoolFalse(ctx context.Context, obj *garmv1alpha1.Pool, reason string, cause error) (ctrl.Result, error) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: reason, Message: cause.Error(), ObservedGeneration: obj.Generation,
	})
	if err := r.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func (r *PoolReconciler) syncRunnerMirrors(ctx context.Context, pool *garmv1alpha1.Pool, instances []garmparams.Instance) error {
	seen := map[string]struct{}{}
	for _, inst := range instances {
		name := runnerMirrorName(pool.Name, inst)
		seen[name] = struct{}{}
		runner := &garmv1alpha1.Runner{}
		key := types.NamespacedName{Namespace: pool.Namespace, Name: name}
		if err := r.Get(ctx, key, runner); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			runner = &garmv1alpha1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: pool.Namespace,
					Labels: map[string]string{
						"garm.igrikus.dev/pool": pool.Name,
					},
					Annotations: map[string]string{
						garmv1alpha1.AnnotationManagedBy: "garm-operator",
					},
				},
				Spec: garmv1alpha1.RunnerSpec{PoolRef: garmv1alpha1.LocalObjectRef{Name: pool.Name}},
			}
			if err := controllerutil.SetControllerReference(pool, runner, r.Scheme); err != nil {
				return err
			}
			if err := r.Create(ctx, runner); err != nil {
				return err
			}
		}
		updateRunnerStatus(runner, inst)
		if err := r.Status().Update(ctx, runner); err != nil {
			return err
		}
	}

	list := &garmv1alpha1.RunnerList{}
	if err := r.List(ctx, list, client.InNamespace(pool.Namespace)); err != nil {
		return err
	}
	for i := range list.Items {
		runner := &list.Items[i]
		if runner.Spec.PoolRef.Name != pool.Name {
			continue
		}
		if _, ok := seen[runner.Name]; ok {
			continue
		}
		if err := r.Delete(ctx, runner); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func updateRunnerStatus(runner *garmv1alpha1.Runner, inst garmparams.Instance) {
	runner.Status.ID = inst.ID
	runner.Status.Name = inst.Name
	runner.Status.ProviderID = inst.ProviderID
	runner.Status.RunnerStatus = string(inst.RunnerStatus)
	runner.Status.AgentID = inst.AgentID
	runner.Status.ObservedGeneration = runner.Generation
	runner.Status.Addresses = runner.Status.Addresses[:0]
	for _, addr := range inst.Addresses {
		runner.Status.Addresses = append(runner.Status.Addresses, addr.Address)
	}
	meta.SetStatusCondition(&runner.Status.Conditions, metav1.Condition{
		Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionTrue,
		Reason: garmv1alpha1.ReasonReconciled, ObservedGeneration: runner.Generation,
	})
}

func runnerMirrorName(poolName string, inst garmparams.Instance) string {
	source := inst.Name
	if source == "" {
		source = inst.ID
	}
	if source == "" {
		source = inst.ProviderID
	}
	name := dnsLabel(poolName + "-" + source)
	if len(name) > 63 {
		name = name[:63]
		name = strings.Trim(name, "-")
	}
	if name == "" {
		return dnsLabel(poolName + "-runner")
	}
	return name
}

func dnsLabel(in string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(in) {
		ok := unicode.IsLetter(r) || unicode.IsDigit(r)
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func buildPoolCreate(s *garmv1alpha1.PoolSpec, imageTag string, templateID uint) garmclient.PoolCreate {
	osType := string(s.OSType)
	if osType == "" {
		osType = "linux"
	}
	osArch := string(s.OSArch)
	if osArch == "" {
		osArch = "amd64"
	}
	tags := append([]string(nil), s.Tags...)
	sort.Strings(tags)
	out := garmclient.PoolCreate{
		ProviderName:           s.ProviderName,
		MaxRunners:             uint(s.MaxRunners),
		MinIdleRunners:         uint(s.MinIdleRunners),
		Image:                  imageTag,
		Flavor:                 s.Flavor,
		OSType:                 osType,
		OSArch:                 osArch,
		Tags:                   tags,
		Enabled:                s.Enabled,
		RunnerBootstrapTimeout: uint(s.RunnerBootstrapTimeoutMinutes),
		GitHubRunnerGroup:      s.GithubRunnerGroup,
		RunnerPrefix:           s.RunnerPrefix,
		Priority:               uint(s.Priority),
		TemplateID:             templateID,
	}
	if s.ExtraSpecs != nil && len(s.ExtraSpecs.Raw) > 0 {
		out.ExtraSpecs = json.RawMessage(s.ExtraSpecs.Raw)
	}
	return out
}

func tagNames(in []garmparams.Tag) []string {
	out := make([]string, 0, len(in))
	for _, t := range in {
		out = append(out, t.Name)
	}
	sort.Strings(out)
	return out
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// poolDiff produces a PoolUpdate containing only the fields that differ
// between actual (from GARM) and desired. Anything unchanged stays nil so
// GARM leaves it alone — that's the no-downtime invariant.
func poolDiff(actual *garmparams.Pool, desired garmclient.PoolCreate) garmclient.PoolUpdate {
	var out garmclient.PoolUpdate
	if actual.Enabled != desired.Enabled {
		v := desired.Enabled
		out.Enabled = &v
	}
	if actual.MaxRunners != desired.MaxRunners {
		v := desired.MaxRunners
		out.MaxRunners = &v
	}
	if actual.MinIdleRunners != desired.MinIdleRunners {
		v := desired.MinIdleRunners
		out.MinIdleRunners = &v
	}
	if actual.RunnerBootstrapTimeout != desired.RunnerBootstrapTimeout {
		v := desired.RunnerBootstrapTimeout
		out.RunnerBootstrapTimeout = &v
	}
	if actual.Image != desired.Image {
		v := desired.Image
		out.Image = &v
	}
	if actual.Flavor != desired.Flavor {
		v := desired.Flavor
		out.Flavor = &v
	}
	if string(actual.OSType) != desired.OSType {
		v := desired.OSType
		out.OSType = &v
	}
	if string(actual.OSArch) != desired.OSArch {
		v := desired.OSArch
		out.OSArch = &v
	}
	if !stringsEqual(tagNames(actual.Tags), desired.Tags) {
		out.Tags = desired.Tags
		if out.Tags == nil {
			out.Tags = []string{}
		}
	}
	if !bytes.Equal(actual.ExtraSpecs, desired.ExtraSpecs) {
		out.ExtraSpecs = desired.ExtraSpecs
		if out.ExtraSpecs == nil {
			out.ExtraSpecs = json.RawMessage("null")
		}
	}
	if actual.GitHubRunnerGroup != desired.GitHubRunnerGroup {
		v := desired.GitHubRunnerGroup
		out.GitHubRunnerGroup = &v
	}
	if actual.RunnerPrefix.Prefix != desired.RunnerPrefix {
		v := desired.RunnerPrefix
		out.RunnerPrefix = &v
	}
	if actual.Priority != desired.Priority {
		v := desired.Priority
		out.Priority = &v
	}
	if desired.TemplateID != 0 && actual.TemplateID != desired.TemplateID {
		v := desired.TemplateID
		out.TemplateID = &v
	}
	return out
}

func (r *PoolReconciler) templateToPools(ctx context.Context, tpl client.Object) []reconcile.Request {
	list := &garmv1alpha1.PoolList{}
	if err := r.List(ctx, list, client.InNamespace(tpl.GetNamespace())); err != nil {
		return nil
	}
	var out []reconcile.Request
	for i := range list.Items {
		p := &list.Items[i]
		ref := p.Spec.RunnerInstallTemplateRef
		if ref != nil && ref.Name == tpl.GetName() {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: p.Namespace, Name: p.Name}})
		}
	}
	return out
}

func (r *PoolReconciler) scopeToPools(ctx context.Context, org client.Object) []reconcile.Request {
	list := &garmv1alpha1.PoolList{}
	if err := r.List(ctx, list, client.InNamespace(org.GetNamespace())); err != nil {
		return nil
	}
	var out []reconcile.Request
	for i := range list.Items {
		p := &list.Items[i]
		if p.Spec.ScopeRef.Name == org.GetName() {
			out = append(out, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: p.Namespace, Name: p.Name}})
		}
	}
	return out
}

// SetupWithManager sets up the controller with the Manager.
func (r *PoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&garmv1alpha1.Pool{}).
		Watches(&garmv1alpha1.RunnerTemplate{}, handler.EnqueueRequestsFromMapFunc(r.templateToPools)).
		Watches(&garmv1alpha1.GiteaOrganization{}, handler.EnqueueRequestsFromMapFunc(r.scopeToPools)).
		Watches(&garmv1alpha1.Repository{}, handler.EnqueueRequestsFromMapFunc(r.scopeToPools)).
		Watches(&garmv1alpha1.Enterprise{}, handler.EnqueueRequestsFromMapFunc(r.scopeToPools)).
		Named("pool").
		Complete(r)
}
