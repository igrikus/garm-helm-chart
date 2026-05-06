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

var _ = Describe("GiteaCredentials Controller", func() {
	Context("When reconciling a resource", func() {
		const credName = "test-creds"
		const epName = "test-ep"
		const secretName = "test-pat"
		ctx := context.Background()
		credNSN := types.NamespacedName{Name: credName, Namespace: "default"}

		AfterEach(func() {
			for _, name := range []string{credName} {
				obj := &garmv1alpha1.GiteaCredentials{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, obj); err == nil {
					obj.Finalizers = nil
					_ = k8sClient.Update(ctx, obj)
					_ = k8sClient.Delete(ctx, obj)
				}
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

		It("creates the credential in GARM after the endpoint is reconciled", func() {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "default"},
				Data:       map[string][]byte{"token": []byte("ghp_dummy")},
			})).To(Succeed())

			ep := &garmv1alpha1.GiteaEndpoint{
				ObjectMeta: metav1.ObjectMeta{Name: epName, Namespace: "default"},
				Spec:       garmv1alpha1.GiteaEndpointSpec{BaseURL: "https://gitea.example.com"},
			}
			Expect(k8sClient.Create(ctx, ep)).To(Succeed())

			gc := fake.New()

			// Drive the endpoint reconciler to populate Status.ID.
			er := &GiteaEndpointReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
			_, err := er.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: epName, Namespace: "default"}})
			Expect(err).NotTo(HaveOccurred())
			_, err = er.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: epName, Namespace: "default"}})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Create(ctx, &garmv1alpha1.GiteaCredentials{
				ObjectMeta: metav1.ObjectMeta{Name: credName, Namespace: "default"},
				Spec: garmv1alpha1.GiteaCredentialsSpec{
					EndpointRef:  garmv1alpha1.LocalObjectRef{Name: epName},
					PATSecretRef: garmv1alpha1.SecretKeyRef{Name: secretName, Key: "token"},
				},
			})).To(Succeed())

			cr := &GiteaCredentialsReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
			_, err = cr.Reconcile(ctx, reconcile.Request{NamespacedName: credNSN})
			Expect(err).NotTo(HaveOccurred())
			_, err = cr.Reconcile(ctx, reconcile.Request{NamespacedName: credNSN})
			Expect(err).NotTo(HaveOccurred())

			Expect(gc.Credentials).To(HaveLen(1))

			obj := &garmv1alpha1.GiteaCredentials{}
			Expect(k8sClient.Get(ctx, credNSN, obj)).To(Succeed())
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
