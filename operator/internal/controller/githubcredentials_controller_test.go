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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
	"github.com/igrikus/garm-helm-chart/operator/internal/garmclient/fake"
)

var _ = Describe("GithubCredentials Controller", func() {
	const namespace = "default"
	ctx := context.Background()
	nsn := types.NamespacedName{Name: "github-creds", Namespace: namespace}
	epNSN := types.NamespacedName{Name: "github-ep", Namespace: namespace}
	secretNSN := types.NamespacedName{Name: "github-token", Namespace: namespace}

	AfterEach(func() {
		for _, obj := range []client.Object{
			&garmv1alpha1.GithubCredentials{}, &garmv1alpha1.GithubEndpoint{}, &corev1.Secret{},
		} {
			name := nsn
			if _, ok := obj.(*garmv1alpha1.GithubEndpoint); ok {
				name = epNSN
			}
			if _, ok := obj.(*corev1.Secret); ok {
				name = secretNSN
			}
			if err := k8sClient.Get(ctx, name, obj); err == nil {
				_ = k8sClient.Delete(ctx, obj)
			}
		}
	})

	It("creates PAT credentials and sets Ready=True", func() {
		gc := fake.New()
		gc.Endpoints[epNSN.Name] = githubEndpointParams(epNSN.Name)
		Expect(createReadyGithubEndpoint(ctx, epNSN)).To(Succeed())
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secretNSN.Name, Namespace: namespace},
			Data:       map[string][]byte{"token": []byte("ghp_secret")},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &garmv1alpha1.GithubCredentials{
			ObjectMeta: metav1.ObjectMeta{Name: nsn.Name, Namespace: namespace},
			Spec: garmv1alpha1.GithubCredentialsSpec{
				EndpointRef:  garmv1alpha1.LocalObjectRef{Name: epNSN.Name},
				AuthType:     garmv1alpha1.AuthTypePAT,
				PATSecretRef: &garmv1alpha1.SecretKeyRef{Name: secretNSN.Name, Key: "token"},
			},
		})).To(Succeed())

		r := &GithubCredentialsReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nsn})
		Expect(err).NotTo(HaveOccurred())

		Expect(gc.Credentials).To(HaveLen(1))
		obj := &garmv1alpha1.GithubCredentials{}
		Expect(k8sClient.Get(ctx, nsn, obj)).To(Succeed())
		Expect(obj.Status.ID).NotTo(BeEmpty())
		Expect(obj.Status.Conditions).To(ContainElement(And(
			HaveField("Type", garmv1alpha1.ConditionReady),
			HaveField("Status", metav1.ConditionTrue),
		)))
	})
})

func createReadyGithubEndpoint(ctx context.Context, nsn types.NamespacedName) error {
	ep := &garmv1alpha1.GithubEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: nsn.Name, Namespace: nsn.Namespace},
		Spec:       garmv1alpha1.GithubEndpointSpec{BaseURL: "https://github.com"},
	}
	if err := k8sClient.Create(ctx, ep); err != nil {
		return err
	}
	if err := k8sClient.Get(ctx, nsn, ep); err != nil {
		return err
	}
	ep.Status.ID = nsn.Name
	return k8sClient.Status().Update(ctx, ep)
}

func createReadyGithubCredentials(ctx context.Context, nsn types.NamespacedName, endpointName string) error {
	creds := &garmv1alpha1.GithubCredentials{
		ObjectMeta: metav1.ObjectMeta{Name: nsn.Name, Namespace: nsn.Namespace},
		Spec: garmv1alpha1.GithubCredentialsSpec{
			EndpointRef: garmv1alpha1.LocalObjectRef{Name: endpointName},
			AuthType:    garmv1alpha1.AuthTypePAT,
			PATSecretRef: &garmv1alpha1.SecretKeyRef{
				Name: "unused-ready-credentials-secret",
				Key:  "token",
			},
		},
	}
	if err := k8sClient.Create(ctx, creds); err != nil {
		return err
	}
	if err := k8sClient.Get(ctx, nsn, creds); err != nil {
		return err
	}
	creds.Status.ID = "1"
	return k8sClient.Status().Update(ctx, creds)
}

func githubEndpointParams(name string) garmparams.ForgeEndpoint {
	return garmparams.ForgeEndpoint{
		Name: name, BaseURL: "https://github.com", APIBaseURL: "https://api.github.com",
		UploadBaseURL: "https://uploads.github.com", EndpointType: garmparams.GithubEndpointType,
	}
}
