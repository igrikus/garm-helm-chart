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

	commonparams "github.com/cloudbase/garm-provider-common/params"
	garmparams "github.com/cloudbase/garm/params"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient/fake"
)

var _ = Describe("Pool Controller", func() {
	const (
		poolName  = "pool-cr"
		orgName   = "pool-org"
		imageName = "pool-image"
		namespace = "default"
	)

	ctx := context.Background()
	poolNSN := types.NamespacedName{Name: poolName, Namespace: namespace}

	AfterEach(func() {
		pool := &garmv1alpha1.Pool{}
		if err := k8sClient.Get(ctx, poolNSN, pool); err == nil {
			pool.Finalizers = nil
			_ = k8sClient.Update(ctx, pool)
			_ = k8sClient.Delete(ctx, pool)
		}
		org := &garmv1alpha1.GiteaOrganization{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: orgName, Namespace: namespace}, org); err == nil {
			org.Finalizers = nil
			_ = k8sClient.Update(ctx, org)
			_ = k8sClient.Delete(ctx, org)
		}
		img := &garmv1alpha1.Image{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: imageName, Namespace: namespace}, img); err == nil {
			_ = k8sClient.Delete(ctx, img)
		}
		tpl := &garmv1alpha1.RunnerTemplate{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: "linux-template", Namespace: namespace}, tpl); err == nil {
			_ = k8sClient.Delete(ctx, tpl)
		}
	})

	newReconciler := func(gc *fake.Client) *PoolReconciler {
		return &PoolReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
	}

	createReadyOrg := func(gc *fake.Client) {
		gc.Orgs["org-1"] = garmparams.Organization{ID: "org-1", Name: "myorg"}
		org := &garmv1alpha1.GiteaOrganization{
			ObjectMeta: metav1.ObjectMeta{Name: orgName, Namespace: namespace},
			Spec:       garmv1alpha1.GiteaOrganizationSpec{Name: "myorg"},
		}
		Expect(k8sClient.Create(ctx, org)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: orgName, Namespace: namespace}, org)).To(Succeed())
		org.Status.ID = "org-1"
		Expect(k8sClient.Status().Update(ctx, org)).To(Succeed())
	}

	createImage := func() {
		Expect(k8sClient.Create(ctx, &garmv1alpha1.Image{
			ObjectMeta: metav1.ObjectMeta{Name: imageName, Namespace: namespace},
			Spec:       garmv1alpha1.ImageSpec{Tag: "ubuntu-24.04"},
		})).To(Succeed())
	}

	createReadyTemplate := func(id string) {
		tpl := &garmv1alpha1.RunnerTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "linux-template", Namespace: namespace},
			Spec: garmv1alpha1.RunnerTemplateSpec{
				OSType:    garmv1alpha1.OSType("linux"),
				ForgeType: garmv1alpha1.ForgeType("gitea"),
				Data:      "install",
			},
		}
		Expect(k8sClient.Create(ctx, tpl)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "linux-template", Namespace: namespace}, tpl)).To(Succeed())
		tpl.Status.ID = id
		tpl.Status.Conditions = []metav1.Condition{{
			Type: garmv1alpha1.ConditionReady, Status: metav1.ConditionTrue,
			Reason: garmv1alpha1.ReasonReconciled, ObservedGeneration: tpl.Generation,
			LastTransitionTime: metav1.Now(),
		}}
		Expect(k8sClient.Status().Update(ctx, tpl)).To(Succeed())
	}

	basePoolSpec := func() garmv1alpha1.PoolSpec {
		return garmv1alpha1.PoolSpec{
			ForgeRef: garmv1alpha1.ForgeRef{Kind: "GiteaEndpoint", Name: "gitea"},
			ScopeRef: garmv1alpha1.ScopeRef{Kind: "GiteaOrganization", Name: orgName},
			ImageRef: garmv1alpha1.LocalObjectRef{Name: imageName},

			ProviderName:                  "lxd",
			Flavor:                        "medium",
			OSType:                        garmv1alpha1.OSType("linux"),
			OSArch:                        garmv1alpha1.OSArch("amd64"),
			Tags:                          []string{"self-hosted", "linux"},
			MinIdleRunners:                1,
			MaxRunners:                    5,
			Enabled:                       true,
			RunnerBootstrapTimeoutMinutes: 20,
			RunnerPrefix:                  "ci",
			Priority:                      10,
		}
	}

	createPool := func(spec garmv1alpha1.PoolSpec) {
		Expect(k8sClient.Create(ctx, &garmv1alpha1.Pool{
			ObjectMeta: metav1.ObjectMeta{Name: poolName, Namespace: namespace},
			Spec:       spec,
		})).To(Succeed())
	}

	It("creates an organization pool and marks it ready", func() {
		gc := fake.New()
		createReadyOrg(gc)
		createImage()
		createPool(basePoolSpec())

		r := newReconciler(gc)
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())

		Expect(gc.Pools).To(HaveLen(1))
		for _, p := range gc.Pools {
			Expect(p.ProviderName).To(Equal("lxd"))
			Expect(p.Image).To(Equal("ubuntu-24.04"))
			Expect(p.MaxRunners).To(Equal(uint(5)))
			Expect(p.OrgID).To(Equal("org-1"))
		}

		obj := &garmv1alpha1.Pool{}
		Expect(k8sClient.Get(ctx, poolNSN, obj)).To(Succeed())
		Expect(obj.Status.ID).NotTo(BeEmpty())
		Expect(obj.Status.Conditions).To(ContainElement(
			And(
				HaveField("Type", garmv1alpha1.ConditionReady),
				HaveField("Status", metav1.ConditionTrue),
			),
		))
	})

	It("adopts a single existing organization pool by runner prefix", func() {
		gc := fake.New()
		createReadyOrg(gc)
		createImage()
		createPool(basePoolSpec())

		gc.Pools["existing-pool"] = garmparams.Pool{
			ID:                     "existing-pool",
			ProviderName:           "lxd",
			MaxRunners:             5,
			MinIdleRunners:         1,
			Image:                  "ubuntu-24.04",
			Flavor:                 "medium",
			OSType:                 commonparams.OSType("linux"),
			OSArch:                 commonparams.OSArch("amd64"),
			Tags:                   []garmparams.Tag{{Name: "linux"}, {Name: "self-hosted"}},
			Enabled:                true,
			RunnerBootstrapTimeout: 20,
			RunnerPrefix:           garmparams.RunnerPrefix{Prefix: "ci"},
			Priority:               10,
			OrgID:                  "org-1",
		}

		r := newReconciler(gc)
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())

		Expect(gc.Pools).To(HaveLen(1))
		obj := &garmv1alpha1.Pool{}
		Expect(k8sClient.Get(ctx, poolNSN, obj)).To(Succeed())
		Expect(obj.Status.ID).To(Equal("existing-pool"))
	})

	It("patches only changed pool fields during drift correction", func() {
		gc := fake.New()
		createReadyOrg(gc)
		createImage()

		spec := basePoolSpec()
		createPool(spec)

		obj := &garmv1alpha1.Pool{}
		Expect(k8sClient.Get(ctx, poolNSN, obj)).To(Succeed())
		controllerutil.AddFinalizer(obj, garmv1alpha1.Finalizer)
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
		Expect(k8sClient.Get(ctx, poolNSN, obj)).To(Succeed())
		obj.Status.ID = "pool-drift"
		Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())

		gc.Pools["pool-drift"] = garmparams.Pool{
			ID:                     "pool-drift",
			ProviderName:           "lxd",
			MaxRunners:             20,
			MinIdleRunners:         1,
			Image:                  "ubuntu-24.04",
			Flavor:                 "medium",
			OSType:                 commonparams.OSType("linux"),
			OSArch:                 commonparams.OSArch("amd64"),
			Tags:                   []garmparams.Tag{{Name: "linux"}, {Name: "self-hosted"}},
			Enabled:                true,
			RunnerBootstrapTimeout: 20,
			RunnerPrefix:           garmparams.RunnerPrefix{Prefix: "ci"},
			Priority:               10,
			OrgID:                  "org-1",
		}

		_, err := newReconciler(gc).Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())

		p := gc.Pools["pool-drift"]
		Expect(p.MaxRunners).To(Equal(uint(5)))
		Expect(p.ProviderName).To(Equal("lxd"))
		Expect(p.Image).To(Equal("ubuntu-24.04"))
		Expect(p.Flavor).To(Equal("medium"))
		Expect(p.RunnerPrefix.Prefix).To(Equal("ci"))
	})

	It("disables and drains runners before deleting the GARM pool", func() {
		gc := fake.New()
		createReadyOrg(gc)
		createImage()
		createPool(basePoolSpec())

		r := newReconciler(gc)
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())

		obj := &garmv1alpha1.Pool{}
		Expect(k8sClient.Get(ctx, poolNSN, obj)).To(Succeed())
		poolID := obj.Status.ID
		gc.Instances[poolID] = []garmparams.Instance{
			{Name: "idle-runner", RunnerStatus: garmparams.RunnerIdle},
			{Name: "active-runner", RunnerStatus: garmparams.RunnerActive},
		}

		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())

		result, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(drainDisabledRequeue))
		Expect(gc.Pools[poolID].Enabled).To(BeFalse())

		result, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(drainWaitRequeue))
		Expect(gc.Instances[poolID]).To(ConsistOf(garmparams.Instance{
			Name: "active-runner", RunnerStatus: garmparams.RunnerActive,
		}))

		gc.Instances[poolID] = nil
		result, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(gc.Pools).NotTo(HaveKey(poolID))

		Eventually(func() bool {
			err := k8sClient.Get(ctx, poolNSN, &garmv1alpha1.Pool{})
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	})

	It("marks Ready=False when the referenced image is missing", func() {
		gc := fake.New()
		createReadyOrg(gc)

		spec := basePoolSpec()
		spec.ImageRef.Name = "missing-image"
		createPool(spec)

		r := newReconciler(gc)
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).To(MatchError(ContainSubstring("image default/missing-image not found")))

		Expect(gc.Pools).To(BeEmpty())

		obj := &garmv1alpha1.Pool{}
		Expect(k8sClient.Get(ctx, poolNSN, obj)).To(Succeed())
		Expect(obj.Status.Conditions).To(ContainElement(
			And(
				HaveField("Type", garmv1alpha1.ConditionReady),
				HaveField("Status", metav1.ConditionFalse),
				HaveField("Reason", garmv1alpha1.ReasonReferenceMiss),
			),
		))
	})

	It("marks Ready=False when the referenced runner template is missing", func() {
		gc := fake.New()
		createReadyOrg(gc)
		createImage()
		spec := basePoolSpec()
		spec.RunnerInstallTemplateRef = &garmv1alpha1.LocalObjectRef{Name: "missing-template"}
		createPool(spec)

		r := newReconciler(gc)
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).To(MatchError(ContainSubstring("runner template default/missing-template not found")))
		Expect(gc.Pools).To(BeEmpty())
	})

	It("passes the runner template ID during pool create", func() {
		gc := fake.New()
		createReadyOrg(gc)
		createImage()
		createReadyTemplate("42")
		spec := basePoolSpec()
		spec.RunnerInstallTemplateRef = &garmv1alpha1.LocalObjectRef{Name: "linux-template"}
		createPool(spec)

		r := newReconciler(gc)
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())

		for _, p := range gc.Pools {
			Expect(p.TemplateID).To(Equal(uint(42)))
			Expect(p.TemplateName).To(Equal(""))
		}
	})

	It("updates pool template drift", func() {
		gc := fake.New()
		createReadyOrg(gc)
		createImage()
		createReadyTemplate("42")
		spec := basePoolSpec()
		spec.RunnerInstallTemplateRef = &garmv1alpha1.LocalObjectRef{Name: "linux-template"}
		createPool(spec)

		obj := &garmv1alpha1.Pool{}
		Expect(k8sClient.Get(ctx, poolNSN, obj)).To(Succeed())
		controllerutil.AddFinalizer(obj, garmv1alpha1.Finalizer)
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
		Expect(k8sClient.Get(ctx, poolNSN, obj)).To(Succeed())
		obj.Status.ID = "pool-template-drift"
		Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())
		gc.Pools["pool-template-drift"] = garmparams.Pool{
			ID: "pool-template-drift", ProviderName: "lxd", MaxRunners: 5, MinIdleRunners: 1,
			Image: "ubuntu-24.04", Flavor: "medium", OSType: commonparams.OSType("linux"),
			OSArch: commonparams.OSArch("amd64"), Tags: []garmparams.Tag{{Name: "linux"}, {Name: "self-hosted"}},
			Enabled: true, RunnerBootstrapTimeout: 20, RunnerPrefix: garmparams.RunnerPrefix{Prefix: "ci"},
			Priority: 10, OrgID: "org-1", TemplateID: 7,
		}

		_, err := newReconciler(gc).Reconcile(ctx, reconcile.Request{NamespacedName: poolNSN})
		Expect(err).NotTo(HaveOccurred())
		Expect(gc.Pools["pool-template-drift"].TemplateID).To(Equal(uint(42)))
	})
})
