/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// GiteaEndpointSpec configures a Gitea forge endpoint that GARM can talk to.
type GiteaEndpointSpec struct {
	// BaseURL is the user-facing URL of the Gitea instance, e.g. https://gitea.example.com.
	// +kubebuilder:validation:Pattern=`^https?://.+`
	BaseURL string `json:"baseURL"`

	// APIBaseURL is the API URL. If empty, BaseURL is used.
	// +optional
	APIBaseURL string `json:"apiBaseURL,omitempty"`

	// +optional
	Description string `json:"description,omitempty"`

	// CACertBundleSecretRef points at a Secret key holding a PEM CA bundle.
	// Required only when the Gitea instance uses a non-public CA.
	// +optional
	CACertBundleSecretRef *SecretKeyRef `json:"caCertBundleSecretRef,omitempty"`
}

type GiteaEndpointStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=gtep
// +kubebuilder:printcolumn:name="BaseURL",type=string,JSONPath=`.spec.baseURL`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type GiteaEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GiteaEndpointSpec   `json:"spec"`
	Status GiteaEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type GiteaEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GiteaEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GiteaEndpoint{}, &GiteaEndpointList{})
}
