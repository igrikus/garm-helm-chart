/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// GiteaOrganizationSpec configures a Gitea organization tracked by GARM.
type GiteaOrganizationSpec struct {
	EndpointRef LocalObjectRef `json:"endpointRef"`

	CredentialsRef LocalObjectRef `json:"credentialsRef"`

	// Name is the Gitea organization name on the forge side.
	Name string `json:"name"`

	// WebhookSecretRef holds the shared secret for incoming Gitea webhooks. Optional;
	// GARM auto-manages the webhook when provided.
	// +optional
	WebhookSecretRef *SecretKeyRef `json:"webhookSecretRef,omitempty"`

	// +optional
	// +kubebuilder:default=roundrobin
	PoolBalancerType PoolBalancerType `json:"poolBalancerType,omitempty"`
}

type GiteaOrganizationStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=gtorg
// +kubebuilder:printcolumn:name="Org",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.spec.endpointRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type GiteaOrganization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GiteaOrganizationSpec   `json:"spec"`
	Status GiteaOrganizationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type GiteaOrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GiteaOrganization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GiteaOrganization{}, &GiteaOrganizationList{})
}
