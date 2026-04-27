/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// EnterpriseSpec configures a GitHub Enterprise tracked by GARM.
// Gitea has no enterprise concept; ForgeRef.Kind must be GithubEndpoint.
// +kubebuilder:validation:XValidation:rule="self.forgeRef.kind == 'GithubEndpoint'",message="Enterprise only supports GithubEndpoint"
type EnterpriseSpec struct {
	ForgeRef ForgeRef `json:"forgeRef"`

	CredentialsRef LocalObjectRef `json:"credentialsRef"`

	// Name is the GitHub Enterprise slug.
	Name string `json:"name"`

	// +optional
	WebhookSecretRef *SecretKeyRef `json:"webhookSecretRef,omitempty"`

	// +optional
	// +kubebuilder:default=roundrobin
	PoolBalancerType PoolBalancerType `json:"poolBalancerType,omitempty"`
}

type EnterpriseStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=gent
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Enterprise struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnterpriseSpec   `json:"spec"`
	Status EnterpriseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type EnterpriseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Enterprise `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Enterprise{}, &EnterpriseList{})
}
