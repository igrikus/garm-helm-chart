/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// GithubAppAuth carries GitHub App credentials.
type GithubAppAuth struct {
	AppID               int64        `json:"appID"`
	InstallationID      int64        `json:"installationID"`
	PrivateKeySecretRef SecretKeyRef `json:"privateKeySecretRef"`
}

// GithubCredentialsSpec wraps either a PAT or a GitHub App for GARM.
// +kubebuilder:validation:XValidation:rule="self.authType != 'pat' || has(self.patSecretRef)",message="patSecretRef is required when authType is pat"
// +kubebuilder:validation:XValidation:rule="self.authType != 'app' || has(self.appAuth)",message="appAuth is required when authType is app"
type GithubCredentialsSpec struct {
	EndpointRef LocalObjectRef `json:"endpointRef"`

	// +optional
	Description string `json:"description,omitempty"`

	AuthType AuthType `json:"authType"`

	// +optional
	PATSecretRef *SecretKeyRef `json:"patSecretRef,omitempty"`

	// +optional
	AppAuth *GithubAppAuth `json:"appAuth,omitempty"`
}

type GithubCredentialsStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=ghcr
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.spec.endpointRef.name`
// +kubebuilder:printcolumn:name="Auth",type=string,JSONPath=`.spec.authType`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type GithubCredentials struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GithubCredentialsSpec   `json:"spec"`
	Status GithubCredentialsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type GithubCredentialsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GithubCredentials `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GithubCredentials{}, &GithubCredentialsList{})
}
