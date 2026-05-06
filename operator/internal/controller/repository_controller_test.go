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

var _ = Describe("Repository Controller", func() {
	const namespace = "default"
	ctx := context.Background()
	repoNSN := types.NamespacedName{Name: "repo-cr", Namespace: namespace}
	epNSN := types.NamespacedName{Name: "repo-github-ep", Namespace: namespace}
	credsNSN := types.NamespacedName{Name: "repo-github-creds", Namespace: namespace}

	AfterEach(func() {
		for _, obj := range []client.Object{
			&garmv1alpha1.Repository{}, &garmv1alpha1.GithubCredentials{}, &garmv1alpha1.GithubEndpoint{},
		} {
			name := repoNSN
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

	It("creates a repository and sets Ready=True", func() {
		Expect(createReadyGithubEndpoint(ctx, epNSN)).To(Succeed())
		Expect(createReadyGithubCredentials(ctx, credsNSN, epNSN.Name)).To(Succeed())
		Expect(k8sClient.Create(ctx, &garmv1alpha1.Repository{
			ObjectMeta: metav1.ObjectMeta{Name: repoNSN.Name, Namespace: namespace},
			Spec: garmv1alpha1.RepositorySpec{
				ForgeRef:       garmv1alpha1.ForgeRef{Kind: "GithubEndpoint", Name: epNSN.Name},
				CredentialsRef: garmv1alpha1.LocalObjectRef{Name: credsNSN.Name},
				Owner:          "octo-org",
				Name:           "octo-repo",
			},
		})).To(Succeed())

		gc := fake.New()
		r := &RepositoryReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Garm: gc}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: repoNSN})
		Expect(err).NotTo(HaveOccurred())
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: repoNSN})
		Expect(err).NotTo(HaveOccurred())

		Expect(gc.Repos).To(HaveLen(1))
		var repoID string
		for _, repo := range gc.Repos {
			repoID = repo.ID
			Expect(repo.WebhookSecret).NotTo(BeEmpty())
		}
		Expect(gc.RepoHooks).To(HaveKey(repoID))

		obj := &garmv1alpha1.Repository{}
		Expect(k8sClient.Get(ctx, repoNSN, obj)).To(Succeed())
		Expect(obj.Status.ID).NotTo(BeEmpty())
		Expect(obj.Status.Conditions).To(ContainElement(And(
			HaveField("Type", garmv1alpha1.ConditionReady),
			HaveField("Status", metav1.ConditionTrue),
		)))
	})
})
