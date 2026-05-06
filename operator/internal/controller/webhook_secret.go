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
	"crypto/rand"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	garmv1alpha1 "github.com/igrikus/garm-helm-chart/operator/api/v1alpha1"
)

func createWebhookSecret(ctx context.Context, c client.Client, namespace string, explicit *garmv1alpha1.SecretKeyRef) (string, error) {
	if explicit != nil {
		return readWebhookSecret(ctx, c, namespace, *explicit)
	}
	return randomWebhookSecret()
}

func readWebhookSecret(ctx context.Context, c client.Client, namespace string, ref garmv1alpha1.SecretKeyRef) (string, error) {
	sec := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, sec); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("webhook secret %s/%s not found", namespace, ref.Name)
		}
		return "", err
	}
	v, ok := sec.Data[ref.Key]
	if !ok || len(v) == 0 {
		return "", fmt.Errorf("webhook secret %s/%s missing key %q", namespace, ref.Name, ref.Key)
	}
	return string(v), nil
}

func randomWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate webhook secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func shouldInstallWebhook(managed bool, install *bool) bool {
	if install != nil {
		return *install
	}
	return managed
}
