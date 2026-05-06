/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	GroupName = "garm.igrikus.dev"

	ConditionReady    = "Ready"
	ConditionSynced   = "Synced"
	ConditionDraining = "Draining"

	ReasonReconciling   = "Reconciling"
	ReasonReconciled    = "Reconciled"
	ReasonReferenceMiss = "ReferenceMissing"
	ReasonAPIError      = "GarmAPIError"

	Finalizer = "garm.igrikus.dev/finalizer"

	AnnotationManagedBy = "garm.igrikus.dev/managed-by"
)

// ForgeRef points at a forge endpoint CR in the same namespace.
type ForgeRef struct {
	// +kubebuilder:validation:Enum=GiteaEndpoint;GithubEndpoint
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// ScopeRef points at the entity that owns a Pool: an organization, repository, or enterprise CR.
type ScopeRef struct {
	// +kubebuilder:validation:Enum=GiteaOrganization;Repository;Enterprise
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// LocalObjectRef is a name-only reference to a CR in the same namespace.
type LocalObjectRef struct {
	Name string `json:"name"`
}

// SecretKeyRef references a single key inside a Secret in the same namespace.
type SecretKeyRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// CommonStatus carries the bookkeeping fields every reconciler writes back.
type CommonStatus struct {
	// ID is the GARM-assigned UUID. Empty before the first successful sync.
	// +optional
	ID string `json:"id,omitempty"`

	// ObservedGeneration is the .metadata.generation last reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// PoolBalancerType selects how GARM picks a pool when several match a job.
// +kubebuilder:validation:Enum=roundrobin;pack
type PoolBalancerType string

const (
	PoolBalancerRoundRobin PoolBalancerType = "roundrobin"
	PoolBalancerPack       PoolBalancerType = "pack"
)

// OSType is the OS family of pool runners.
// +kubebuilder:validation:Enum=linux;windows
type OSType string

// OSArch is the CPU arch of pool runners.
// +kubebuilder:validation:Enum=amd64;arm64
type OSArch string

// AuthType selects credential mode for GitHub.
// +kubebuilder:validation:Enum=pat;app
type AuthType string

const (
	AuthTypePAT AuthType = "pat"
	AuthTypeApp AuthType = "app"
)
