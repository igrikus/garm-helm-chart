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

var _ = Describe("ServerSettings Controller", func() {
	const namespace = "default"
	ctx := context.Background()
	nsn := types.NamespacedName{Name: "garm", Namespace: namespace}

	AfterEach(func() {
		obj := &garmv1alpha1.ServerSettings{}
		if err := k8sClient.Get(ctx, nsn, obj); err == nil {
			_ = k8sClient.Delete(ctx, obj)
		}
		sec := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: "controller-ca", Namespace: namespace}, sec); err == nil {
			_ = k8sClient.Delete(ctx, sec)
		}
	})

	createSettingsCR := func(spec garmv1alpha1.ServerSettingsSpec) {
		Expect(k8sClient.Create(ctx, &garmv1alpha1.ServerSettings{
			ObjectMeta: metav1.ObjectMeta{Name: nsn.Name, Namespace: namespace},
			Spec:       spec,
		})).To(Succeed())
	}

	reconcileOnce := func(gc *fake.Client) error {
		_, err := (&ServerSettingsReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}).Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		return err
	}

	stringPtr := func(v string) *string { return &v }
	boolPtr := func(v bool) *bool { return &v }
	uintPtr := func(v uint) *uint { return &v }

	It("updates configured settings and preserves omitted backoff", func() {
		gc := fake.New()
		gc.ServerSettings.MinimumJobAgeBackoff = 30
		createSettingsCR(garmv1alpha1.ServerSettingsSpec{
			MetadataURL:          stringPtr("https://garm.example.com/api/v1/metadata"),
			CallbackURL:          stringPtr("https://garm.example.com/api/v1/callbacks"),
			WebhookURL:           stringPtr("https://garm.example.com/webhooks"),
			AgentURL:             stringPtr("https://garm.example.com/agent"),
			GARMAgentReleasesURL: stringPtr("https://api.github.com/repos/cloudbase/garm-agent/releases"),
			SyncGARMAgentTools:   boolPtr(true),
		})

		Expect(reconcileOnce(gc)).To(Succeed())
		Expect(gc.ServerSettings.MetadataURL).To(Equal("https://garm.example.com/api/v1/metadata"))
		Expect(gc.ServerSettings.CallbackURL).To(Equal("https://garm.example.com/api/v1/callbacks"))
		Expect(gc.ServerSettings.WebhookURL).To(Equal("https://garm.example.com/webhooks"))
		Expect(gc.ServerSettings.AgentURL).To(Equal("https://garm.example.com/agent"))
		Expect(gc.ServerSettings.GARMAgentReleasesURL).To(Equal("https://api.github.com/repos/cloudbase/garm-agent/releases"))
		Expect(gc.ServerSettings.SyncGARMAgentTools).To(BeTrue())
		Expect(gc.ServerSettings.MinimumJobAgeBackoff).To(Equal(uint(30)))
	})

	It("applies explicit zero minimum job age backoff", func() {
		gc := fake.New()
		gc.ServerSettings.MinimumJobAgeBackoff = 30
		createSettingsCR(garmv1alpha1.ServerSettingsSpec{
			MinimumJobAgeBackoffSeconds: uintPtr(0),
		})

		Expect(reconcileOnce(gc)).To(Succeed())
		Expect(gc.ServerSettings.MinimumJobAgeBackoff).To(Equal(uint(0)))
	})

	It("sets CA bundle from a secret", func() {
		gc := fake.New()
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "controller-ca", Namespace: namespace},
			Data: map[string][]byte{
				"ca.crt": []byte("test-ca"),
			},
		})).To(Succeed())
		createSettingsCR(garmv1alpha1.ServerSettingsSpec{
			CACertBundleSecretRef: &garmv1alpha1.SecretKeyRef{Name: "controller-ca", Key: "ca.crt"},
		})

		Expect(reconcileOnce(gc)).To(Succeed())
		Expect(gc.ServerSettings.CACertBundle).To(Equal([]byte("test-ca")))
	})

	It("clears CA bundle when secret ref is omitted", func() {
		gc := fake.New()
		gc.ServerSettings.CACertBundle = []byte("old-ca")
		createSettingsCR(garmv1alpha1.ServerSettingsSpec{})

		Expect(reconcileOnce(gc)).To(Succeed())
		Expect(gc.ServerSettings.CACertBundle).To(BeNil())
	})

	It("marks not ready when the CA bundle secret is missing", func() {
		gc := fake.New()
		createSettingsCR(garmv1alpha1.ServerSettingsSpec{
			CACertBundleSecretRef: &garmv1alpha1.SecretKeyRef{Name: "controller-ca", Key: "ca.crt"},
		})

		Expect(reconcileOnce(gc)).To(HaveOccurred())
		obj := &garmv1alpha1.ServerSettings{}
		Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
		Expect(obj.Status.Conditions).To(ContainElement(And(
			HaveField("Type", garmv1alpha1.ConditionReady),
			HaveField("Status", metav1.ConditionFalse),
			HaveField("Reason", garmv1alpha1.ReasonReferenceMiss),
		)))
	})
})
