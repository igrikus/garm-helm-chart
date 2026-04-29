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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient/fake"
)

var _ = Describe("Enterprise Controller", func() {
	const namespace = "default"
	ctx := context.Background()
	entNSN := types.NamespacedName{Name: "enterprise-cr", Namespace: namespace}
	epNSN := types.NamespacedName{Name: "enterprise-github-ep", Namespace: namespace}
	credsNSN := types.NamespacedName{Name: "enterprise-github-creds", Namespace: namespace}

	AfterEach(func() {
		for _, obj := range []client.Object{
			&garmv1alpha1.Enterprise{}, &garmv1alpha1.GithubCredentials{}, &garmv1alpha1.GithubEndpoint{},
		} {
			name := entNSN
			if _, ok := obj.(*garmv1alpha1.GithubCredentials); ok {
				name = credsNSN
			}
			if _, ok := obj.(*garmv1alpha1.GithubEndpoint); ok {
				name = epNSN
			}
			if err := k8sClient.Get(ctx, name, obj); err == nil {
				obj.SetFinalizers(nil)
				_ = k8sClient.Update(ctx, obj)
				_ = k8sClient.Delete(ctx, obj)
			}
		}
	})

	It("creates an enterprise and sets Ready=True", func() {
		Expect(createReadyGithubEndpoint(ctx, epNSN)).To(Succeed())
		Expect(createReadyGithubCredentials(ctx, credsNSN, epNSN.Name)).To(Succeed())
		Expect(k8sClient.Create(ctx, &garmv1alpha1.Enterprise{
			ObjectMeta: metav1.ObjectMeta{Name: entNSN.Name, Namespace: namespace},
			Spec: garmv1alpha1.EnterpriseSpec{
				ForgeRef:       garmv1alpha1.ForgeRef{Kind: "GithubEndpoint", Name: epNSN.Name},
				CredentialsRef: garmv1alpha1.LocalObjectRef{Name: credsNSN.Name},
				Name:           "octo-enterprise",
			},
		})).To(Succeed())

		gc := fake.New()
		r := &EnterpriseReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: entNSN})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: entNSN})
		Expect(err).NotTo(HaveOccurred())

		Expect(gc.Enterprises).To(HaveLen(1))
		obj := &garmv1alpha1.Enterprise{}
		Expect(k8sClient.Get(ctx, entNSN, obj)).To(Succeed())
		Expect(obj.Status.ID).NotTo(BeEmpty())
		Expect(obj.Status.Conditions).To(ContainElement(And(
			HaveField("Type", garmv1alpha1.ConditionReady),
			HaveField("Status", metav1.ConditionTrue),
		)))
	})
})
