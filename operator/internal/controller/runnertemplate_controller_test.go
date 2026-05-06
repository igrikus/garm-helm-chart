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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient/fake"
)

var _ = Describe("RunnerTemplate Controller", func() {
	const namespace = "default"
	ctx := context.Background()
	nsn := types.NamespacedName{Name: "linux-github", Namespace: namespace}

	AfterEach(func() {
		obj := &garmv1alpha1.RunnerTemplate{}
		if err := k8sClient.Get(ctx, nsn, obj); err == nil {
			obj.Finalizers = nil
			_ = k8sClient.Update(ctx, obj)
			_ = k8sClient.Delete(ctx, obj)
		}
	})

	createTemplateCR := func() {
		Expect(k8sClient.Create(ctx, &garmv1alpha1.RunnerTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: nsn.Name, Namespace: namespace},
			Spec: garmv1alpha1.RunnerTemplateSpec{
				Description: "linux template",
				OSType:      garmv1alpha1.OSType("linux"),
				ForgeType:   garmv1alpha1.ForgeType("github"),
				Data:        "#!/bin/bash\necho ready\n",
			},
		})).To(Succeed())
	}

	It("creates a template and marks it ready", func() {
		gc := fake.New()
		createTemplateCR()
		r := &RunnerTemplateReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())

		Expect(gc.Templates).To(HaveLen(1))
		obj := &garmv1alpha1.RunnerTemplate{}
		Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
		Expect(obj.Status.ID).To(Equal("1"))
		Expect(obj.Status.Conditions).To(ContainElement(And(
			HaveField("Type", garmv1alpha1.ConditionReady),
			HaveField("Status", metav1.ConditionTrue),
		)))
	})

	It("adopts and updates an existing matching template", func() {
		gc := fake.New()
		gc.Templates[7] = garmparams.Template{
			ID: 7, Name: nsn.Name, Description: "old", OSType: commonparams.OSType("linux"),
			ForgeType: garmparams.GithubEndpointType, Data: []byte("old"),
		}
		createTemplateCR()
		r := &RunnerTemplateReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())

		Expect(gc.Templates[7].Description).To(Equal("linux template"))
		Expect(string(gc.Templates[7].Data)).To(Equal("#!/bin/bash\necho ready\n"))
	})

	It("deletes the GARM template on finalizer cleanup", func() {
		gc := fake.New()
		createTemplateCR()
		r := &RunnerTemplateReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())

		obj := &garmv1alpha1.RunnerTemplate{}
		Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())
		Expect(gc.Templates).To(BeEmpty())
	})

	It("recreates when immutable forge or OS changes", func() {
		gc := fake.New()
		createTemplateCR()
		obj := &garmv1alpha1.RunnerTemplate{}
		Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
		controllerutil.AddFinalizer(obj, garmv1alpha1.Finalizer)
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
		Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
		obj.Status.ID = "3"
		Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())
		gc.Templates[3] = garmparams.Template{
			ID: 3, Name: nsn.Name, OSType: commonparams.OSType("windows"),
			ForgeType: garmparams.GithubEndpointType, Data: []byte("old"),
		}

		_, err := (&RunnerTemplateReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}).Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())
		Expect(gc.Templates).NotTo(HaveKey(uint(3)))
		Expect(gc.Templates).To(HaveKey(uint(1)))
	})
})
