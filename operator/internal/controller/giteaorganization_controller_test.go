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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient/fake"
)

var _ = Describe("GiteaOrganization Controller", func() {
	Context("When reconciling a resource", func() {
		const epName = "org-ep"
		const credName = "org-cred"
		const secretName = "org-pat"
		const orgName = "org-cr"
		ctx := context.Background()
		nsn := types.NamespacedName{Name: orgName, Namespace: "default"}

		AfterEach(func() {
			for _, n := range []string{orgName} {
				obj := &garmv1alpha1.GiteaOrganization{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: n, Namespace: "default"}, obj); err == nil {
					obj.Finalizers = nil
					_ = k8sClient.Update(ctx, obj)
					_ = k8sClient.Delete(ctx, obj)
				}
			}
			cred := &garmv1alpha1.GiteaCredentials{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: credName, Namespace: "default"}, cred); err == nil {
				cred.Finalizers = nil
				_ = k8sClient.Update(ctx, cred)
				_ = k8sClient.Delete(ctx, cred)
			}
			ep := &garmv1alpha1.GiteaEndpoint{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: epName, Namespace: "default"}, ep); err == nil {
				ep.Finalizers = nil
				_ = k8sClient.Update(ctx, ep)
				_ = k8sClient.Delete(ctx, ep)
			}
			sec := &corev1.Secret{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: "default"}, sec); err == nil {
				_ = k8sClient.Delete(ctx, sec)
			}
		})

		It("creates the org in GARM with credentials_name wired up", func() {
			gc := fake.New()

			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "default"},
				Data:       map[string][]byte{"token": []byte("ghp_dummy")},
			})).To(Succeed())

			Expect(k8sClient.Create(ctx, &garmv1alpha1.GiteaEndpoint{
				ObjectMeta: metav1.ObjectMeta{Name: epName, Namespace: "default"},
				Spec:       garmv1alpha1.GiteaEndpointSpec{BaseURL: "https://gitea.example.com"},
			})).To(Succeed())
			er := &GiteaEndpointReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
			epNSN := types.NamespacedName{Name: epName, Namespace: "default"}
			_, err := er.Reconcile(ctx, reconcile.Request{NamespacedName: epNSN})
			Expect(err).NotTo(HaveOccurred())
			_, err = er.Reconcile(ctx, reconcile.Request{NamespacedName: epNSN})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Create(ctx, &garmv1alpha1.GiteaCredentials{
				ObjectMeta: metav1.ObjectMeta{Name: credName, Namespace: "default"},
				Spec: garmv1alpha1.GiteaCredentialsSpec{
					EndpointRef:  garmv1alpha1.LocalObjectRef{Name: epName},
					PATSecretRef: garmv1alpha1.SecretKeyRef{Name: secretName, Key: "token"},
				},
			})).To(Succeed())
			cr := &GiteaCredentialsReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
			credNSN := types.NamespacedName{Name: credName, Namespace: "default"}
			_, err = cr.Reconcile(ctx, reconcile.Request{NamespacedName: credNSN})
			Expect(err).NotTo(HaveOccurred())
			_, err = cr.Reconcile(ctx, reconcile.Request{NamespacedName: credNSN})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Create(ctx, &garmv1alpha1.GiteaOrganization{
				ObjectMeta: metav1.ObjectMeta{Name: orgName, Namespace: "default"},
				Spec: garmv1alpha1.GiteaOrganizationSpec{
					EndpointRef:    garmv1alpha1.LocalObjectRef{Name: epName},
					CredentialsRef: garmv1alpha1.LocalObjectRef{Name: credName},
					Name:           "myorg",
				},
			})).To(Succeed())

			or := &GiteaOrganizationReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
			_, err = or.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
			Expect(err).NotTo(HaveOccurred())
			_, err = or.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
			Expect(err).NotTo(HaveOccurred())

			Expect(gc.Orgs).To(HaveLen(1))
			var orgID string
			for _, o := range gc.Orgs {
				orgID = o.ID
				Expect(o.Name).To(Equal("myorg"))
				Expect(o.CredentialsName).To(Equal(credName))
				Expect(o.WebhookSecret).NotTo(BeEmpty())
			}
			Expect(gc.OrgHooks).To(HaveKey(orgID))

			obj := &garmv1alpha1.GiteaOrganization{}
			Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
			Expect(obj.Status.ID).NotTo(BeEmpty())
			Expect(obj.Status.Conditions).To(ContainElement(
				And(
					HaveField("Type", garmv1alpha1.ConditionReady),
					HaveField("Status", metav1.ConditionTrue),
				),
			))
		})
	})
})
