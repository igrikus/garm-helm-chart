/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ForgeType is the forge family a runner template is valid for.
// +kubebuilder:validation:Enum=github;gitea
type ForgeType string

// RunnerTemplateSpec configures a GARM runner install template. The Kubernetes
// object name is used as the template name in GARM.
type RunnerTemplateSpec struct {
	// +optional
	Description string `json:"description,omitempty"`

	// +kubebuilder:default=linux
	OSType OSType `json:"osType,omitempty"`

	ForgeType ForgeType `json:"forgeType"`

	Data string `json:"data"`

	// ExtraContext defines key-value pairs injected into extraSpecs.extra_context
	// for every pool that references this template. Values can be inline or resolved
	// from Kubernetes Secrets. Pool-level extra_context keys take precedence.
	// +optional
	ExtraContext map[string]ExtraContextEntry `json:"extraContext,omitempty"`
}

// RunnerTemplateStatus reports observed template state.
type RunnerTemplateStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=grt
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.id`
// +kubebuilder:printcolumn:name="Forge",type=string,JSONPath=`.spec.forgeType`
// +kubebuilder:printcolumn:name="OS",type=string,JSONPath=`.spec.osType`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type RunnerTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RunnerTemplateSpec   `json:"spec"`
	Status RunnerTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type RunnerTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RunnerTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RunnerTemplate{}, &RunnerTemplateList{})
}
