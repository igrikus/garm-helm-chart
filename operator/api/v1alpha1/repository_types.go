/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RepositorySpec configures a single repository tracked by GARM. Forge-agnostic
// via ForgeRef.
type RepositorySpec struct {
	ForgeRef ForgeRef `json:"forgeRef"`

	CredentialsRef LocalObjectRef `json:"credentialsRef"`

	// Owner is the org/user namespace on the forge.
	Owner string `json:"owner"`

	// Name is the repository name on the forge.
	Name string `json:"name"`

	// WebhookSecretRef holds the shared secret for incoming forge webhooks. Optional;
	// when omitted, the operator generates a transient secret and GARM stores it encrypted.
	// +optional
	WebhookSecretRef *SecretKeyRef `json:"webhookSecretRef,omitempty"`

	// InstallWebhook asks GARM to install/manage the forge webhook for this repository.
	// When omitted, it defaults to true only when webhookSecretRef is omitted.
	// +optional
	InstallWebhook *bool `json:"installWebhook,omitempty"`

	// WebhookInsecureSSL disables SSL verification for the installed forge webhook.
	// +optional
	WebhookInsecureSSL bool `json:"webhookInsecureSSL,omitempty"`

	// +optional
	// +kubebuilder:default=roundrobin
	PoolBalancerType PoolBalancerType `json:"poolBalancerType,omitempty"`
}

type RepositoryStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=grepo
// +kubebuilder:printcolumn:name="Owner",type=string,JSONPath=`.spec.owner`
// +kubebuilder:printcolumn:name="Repo",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Forge",type=string,JSONPath=`.spec.forgeRef.kind`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositorySpec   `json:"spec"`
	Status RepositoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Repository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Repository{}, &RepositoryList{})
}
