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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient/fake"
)

var _ = Describe("GiteaEndpoint Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-endpoint"
		ctx := context.Background()
		nsn := types.NamespacedName{Name: resourceName, Namespace: "default"}

		AfterEach(func() {
			obj := &garmv1alpha1.GiteaEndpoint{}
			if err := k8sClient.Get(ctx, nsn, obj); err == nil {
				obj.Finalizers = nil
				_ = k8sClient.Update(ctx, obj)
				_ = k8sClient.Delete(ctx, obj)
			}
		})

		It("creates the endpoint in GARM and sets Ready=True", func() {
			Expect(k8sClient.Create(ctx, &garmv1alpha1.GiteaEndpoint{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
				Spec: garmv1alpha1.GiteaEndpointSpec{
					BaseURL:                  "https://gitea.example.com",
					ToolsMetadataURL:         "https://gitea.example.com/api/v1/repos/gitea/act_runner/releases",
					UseInternalToolsMetadata: true,
				},
			})).To(Succeed())

			gc := fake.New()
			r := &GiteaEndpointReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}

			// 1st reconcile: adds finalizer (returns Requeue).
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
			Expect(err).NotTo(HaveOccurred())
			// 2nd reconcile: creates the endpoint in GARM.
			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
			Expect(err).NotTo(HaveOccurred())

			Expect(gc.Endpoints).To(HaveKey(resourceName))
			Expect(gc.Endpoints[resourceName].ToolsMetadataURL).To(Equal("https://gitea.example.com/api/v1/repos/gitea/act_runner/releases"))
			Expect(gc.Endpoints[resourceName].UseInternalToolsMetadata).NotTo(BeNil())
			Expect(*gc.Endpoints[resourceName].UseInternalToolsMetadata).To(BeTrue())

			obj := &garmv1alpha1.GiteaEndpoint{}
			Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
			Expect(obj.Status.ID).To(Equal(resourceName))
			Expect(obj.Status.Conditions).To(ContainElement(
				And(
					HaveField("Type", garmv1alpha1.ConditionReady),
					HaveField("Status", metav1.ConditionTrue),
				),
			))
		})
	})
})
