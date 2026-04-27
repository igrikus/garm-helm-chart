/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PoolSpec configures a runner pool. A pool is bound to one forge (ForgeRef)
// and one entity scope (ScopeRef: GiteaOrganization | Repository | Enterprise).
// +kubebuilder:validation:XValidation:rule="self.minIdleRunners <= self.maxRunners",message="minIdleRunners must be <= maxRunners"
type PoolSpec struct {
	ForgeRef ForgeRef `json:"forgeRef"`

	ScopeRef ScopeRef `json:"scopeRef"`

	// ProviderName is the GARM provider this pool dispatches to (lxd, gcp, ...).
	ProviderName string `json:"providerName"`

	// ImageRef points at an Image CR in the same namespace. Indirection makes
	// "bump every pool to a new image" a single edit.
	ImageRef LocalObjectRef `json:"imageRef"`

	Flavor string `json:"flavor"`

	// +optional
	// +kubebuilder:default=linux
	OSType OSType `json:"osType,omitempty"`

	// +optional
	// +kubebuilder:default=amd64
	OSArch OSArch `json:"osArch,omitempty"`

	// +optional
	Tags []string `json:"tags,omitempty"`

	// +optional
	// +kubebuilder:default=0
	MinIdleRunners uint32 `json:"minIdleRunners,omitempty"`

	// +optional
	// +kubebuilder:default=5
	MaxRunners uint32 `json:"maxRunners,omitempty"`

	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// +optional
	// +kubebuilder:default=20
	RunnerBootstrapTimeoutMinutes uint32 `json:"runnerBootstrapTimeoutMinutes,omitempty"`

	// +optional
	RunnerPrefix string `json:"runnerPrefix,omitempty"`

	// GithubRunnerGroup is GitHub-only; ignored for Gitea pools.
	// +optional
	GithubRunnerGroup string `json:"githubRunnerGroup,omitempty"`

	// +optional
	// +kubebuilder:default=0
	Priority uint32 `json:"priority,omitempty"`

	// ExtraSpecs is a provider-specific JSON blob, passed verbatim to GARM.
	// +optional
	ExtraSpecs *apiextv1.JSON `json:"extraSpecs,omitempty"`

	// RunnerInstallTemplateRef references a runner install template by name (managed elsewhere).
	// +optional
	RunnerInstallTemplateRef *LocalObjectRef `json:"runnerInstallTemplateRef,omitempty"`
}

// PoolStatus reports observed pool state.
type PoolStatus struct {
	CommonStatus `json:",inline"`

	// IdleRunners is the count of currently idle runners. Surfaced for the scale subresource.
	// +optional
	IdleRunners uint32 `json:"idleRunners,omitempty"`

	// Selector is consumed by the scale subresource. Always empty for now (no label selection).
	// +optional
	Selector string `json:"selector,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.minIdleRunners,statuspath=.status.idleRunners,selectorpath=.status.selector
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=gpool
// +kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.id`
// +kubebuilder:printcolumn:name="Min",type=integer,JSONPath=`.spec.minIdleRunners`
// +kubebuilder:printcolumn:name="Max",type=integer,JSONPath=`.spec.maxRunners`
// +kubebuilder:printcolumn:name="Provider",type=string,JSONPath=`.spec.providerName`
// +kubebuilder:printcolumn:name="Scope",type=string,JSONPath=`.spec.scopeRef.kind`
// +kubebuilder:printcolumn:name="ScopeName",type=string,JSONPath=`.spec.scopeRef.name`
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Pool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PoolSpec   `json:"spec"`
	Status PoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type PoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Pool{}, &PoolList{})
}
