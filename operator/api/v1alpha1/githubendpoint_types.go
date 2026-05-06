/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// GithubEndpointSpec configures a GitHub (or GHES) forge endpoint.
type GithubEndpointSpec struct {
	// +kubebuilder:validation:Pattern=`^https?://.+`
	BaseURL string `json:"baseURL"`

	// +optional
	APIBaseURL string `json:"apiBaseURL,omitempty"`

	// UploadBaseURL is used for release/asset uploads (GHES quirk).
	// +optional
	UploadBaseURL string `json:"uploadBaseURL,omitempty"`

	// +optional
	Description string `json:"description,omitempty"`

	// +optional
	CACertBundleSecretRef *SecretKeyRef `json:"caCertBundleSecretRef,omitempty"`
}

type GithubEndpointStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=ghep
// +kubebuilder:printcolumn:name="BaseURL",type=string,JSONPath=`.spec.baseURL`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type GithubEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GithubEndpointSpec   `json:"spec"`
	Status GithubEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type GithubEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GithubEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GithubEndpoint{}, &GithubEndpointList{})
}
