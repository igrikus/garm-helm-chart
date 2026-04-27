/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RunnerSpec is intentionally minimal. Runner is a read-mostly mirror of GARM's
// runner inventory so engineers can `kubectl get runners` and inspect status,
// not a way to declaratively create runners (those are spawned by Pool reconcile).
type RunnerSpec struct {
	// PoolRef points at the owning Pool CR.
	PoolRef LocalObjectRef `json:"poolRef"`
}

// RunnerStatus mirrors fields from `garm-cli runner show`.
type RunnerStatus struct {
	CommonStatus `json:",inline"`

	// Name is the runner's name as registered with the forge.
	// +optional
	Name string `json:"name,omitempty"`

	// ProviderID is the cloud-provider-side instance identifier.
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// RunnerStatus is GARM's view ("idle", "active", "pending", ...).
	// +optional
	RunnerStatus string `json:"runnerStatus,omitempty"`

	// AgentID is the GitHub/Gitea-assigned runner agent ID.
	// +optional
	AgentID int64 `json:"agentID,omitempty"`

	// Addresses lists the runner's network addresses.
	// +optional
	Addresses []string `json:"addresses,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=grun
// +kubebuilder:printcolumn:name="Pool",type=string,JSONPath=`.spec.poolRef.name`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.runnerStatus`
// +kubebuilder:printcolumn:name="Provider-ID",type=string,JSONPath=`.status.providerID`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Runner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RunnerSpec   `json:"spec"`
	Status RunnerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type RunnerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Runner `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Runner{}, &RunnerList{})
}
