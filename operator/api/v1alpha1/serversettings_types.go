/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ServerSettingsSpec configures GARM server/controller settings.
type ServerSettingsSpec struct {
	// MetadataURL is the public metadata URL used by runners during bootstrap.
	// +optional
	// +kubebuilder:validation:Pattern=`^https?://.+`
	MetadataURL *string `json:"metadataURL,omitempty"`

	// CallbackURL is the public callback URL used by runners to report status.
	// +optional
	// +kubebuilder:validation:Pattern=`^https?://.+`
	CallbackURL *string `json:"callbackURL,omitempty"`

	// WebhookURL is the public webhook base URL used by GitHub/Gitea.
	// +optional
	// +kubebuilder:validation:Pattern=`^https?://.+`
	WebhookURL *string `json:"webhookURL,omitempty"`

	// AgentURL is the public agent websocket URL used by agent-mode runners.
	// +optional
	// +kubebuilder:validation:Pattern=`^https?://.+`
	AgentURL *string `json:"agentURL,omitempty"`

	// GARMAgentReleasesURL is the URL used to sync garm-agent release metadata.
	// +optional
	// +kubebuilder:validation:Pattern=`^https?://.+`
	GARMAgentReleasesURL *string `json:"garmAgentReleasesURL,omitempty"`

	// SyncGARMAgentTools enables or disables automatic garm-agent tools sync.
	// +optional
	SyncGARMAgentTools *bool `json:"syncGARMAgentTools,omitempty"`

	// MinimumJobAgeBackoffSeconds is the minimum queued-job age before GARM allocates a runner.
	// Set to 0 for immediate reaction. Omit to preserve GARM's current/server-side value.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MinimumJobAgeBackoffSeconds *uint `json:"minimumJobAgeBackoffSeconds,omitempty"`

	// CACertBundleSecretRef points at a Secret key holding a PEM CA bundle.
	// If omitted, the operator clears any existing GARM controller CA bundle.
	// +optional
	CACertBundleSecretRef *SecretKeyRef `json:"caCertBundleSecretRef,omitempty"`
}

type ServerSettingsStatus struct {
	CommonStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=garm,shortName=gss
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=='Ready')].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ServerSettings struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServerSettingsSpec   `json:"spec"`
	Status ServerSettingsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ServerSettingsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServerSettings `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServerSettings{}, &ServerSettingsList{})
}
