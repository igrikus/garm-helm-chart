/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// GiteaCredentialsSpec wraps a Gitea PAT used by GARM.
type GiteaCredentialsSpec struct {
	// EndpointRef points to the GiteaEndpoint this credential authenticates against.
	EndpointRef LocalObjectRef `json:"endpointRef"`

	// +optional
	Description string `json:"description,omitempty"`

	// PATSecretRef references a Secret key holding the personal access token.
	PATSecretRef SecretKeyRef `json:"patSecretRef"`
}

type GiteaCredentialsStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=gtcr
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.spec.endpointRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type GiteaCredentials struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GiteaCredentialsSpec   `json:"spec"`
	Status GiteaCredentialsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type GiteaCredentialsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GiteaCredentials `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GiteaCredentials{}, &GiteaCredentialsList{})
}
