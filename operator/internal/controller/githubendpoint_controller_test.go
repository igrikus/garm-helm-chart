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

	garmparams "github.com/cloudbase/garm/params"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient/fake"
)

var _ = Describe("GithubEndpoint Controller", func() {
	const namespace = "default"
	ctx := context.Background()
	nsn := types.NamespacedName{Name: "github-endpoint", Namespace: namespace}

	AfterEach(func() {
		obj := &garmv1alpha1.GithubEndpoint{}
		if err := k8sClient.Get(ctx, nsn, obj); err == nil {
			obj.Finalizers = nil
			_ = k8sClient.Update(ctx, obj)
			_ = k8sClient.Delete(ctx, obj)
		}
	})

	It("creates the GitHub endpoint in GARM and sets Ready=True", func() {
		Expect(k8sClient.Create(ctx, &garmv1alpha1.GithubEndpoint{
			ObjectMeta: metav1.ObjectMeta{Name: nsn.Name, Namespace: namespace},
			Spec: garmv1alpha1.GithubEndpointSpec{
				BaseURL:     "https://github.com",
				Description: "public github",
			},
		})).To(Succeed())

		gc := fake.New()
		r := &GithubEndpointReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())

		Expect(gc.Endpoints).To(HaveKey(nsn.Name))
		Expect(gc.Endpoints[nsn.Name].APIBaseURL).To(Equal("https://api.github.com"))
		Expect(gc.Endpoints[nsn.Name].UploadBaseURL).To(Equal("https://uploads.github.com"))

		obj := &garmv1alpha1.GithubEndpoint{}
		Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
		Expect(obj.Status.ID).To(Equal(nsn.Name))
		Expect(obj.Status.Conditions).To(ContainElement(And(
			HaveField("Type", garmv1alpha1.ConditionReady),
			HaveField("Status", metav1.ConditionTrue),
		)))
	})

	It("adopts an existing GitHub endpoint when GARM reports create conflict", func() {
		Expect(k8sClient.Create(ctx, &garmv1alpha1.GithubEndpoint{
			ObjectMeta: metav1.ObjectMeta{Name: nsn.Name, Namespace: namespace},
			Spec: garmv1alpha1.GithubEndpointSpec{
				BaseURL:     "https://github.com",
				Description: "public github",
			},
		})).To(Succeed())

		gc := fake.New()
		gc.Endpoints[nsn.Name] = garmparams.ForgeEndpoint{
			Name:       nsn.Name,
			BaseURL:    "https://old-github.example.com",
			APIBaseURL: "https://old-github.example.com/api/v3",
		}
		r := &GithubEndpointReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())

		obj := &garmv1alpha1.GithubEndpoint{}
		Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
		Expect(obj.Status.ID).To(Equal(nsn.Name))
		Expect(obj.Status.Conditions).To(ContainElement(And(
			HaveField("Type", garmv1alpha1.ConditionReady),
			HaveField("Status", metav1.ConditionTrue),
		)))
		Expect(gc.Endpoints[nsn.Name].BaseURL).To(Equal("https://github.com"))
		Expect(gc.Endpoints[nsn.Name].APIBaseURL).To(Equal("https://api.github.com"))
		Expect(gc.Endpoints[nsn.Name].UploadBaseURL).To(Equal("https://uploads.github.com"))
	})
})
