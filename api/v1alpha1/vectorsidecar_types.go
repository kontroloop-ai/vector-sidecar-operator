/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VectorSidecarSpec defines the desired state of VectorSidecar
type VectorSidecarSpec struct {
	// Enabled controls whether sidecar injection is active for this VectorSidecar
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Selector defines label selectors for matching target Deployments
	// +kubebuilder:validation:Required
	Selector metav1.LabelSelector `json:"selector"`

	// Sidecar defines the Vector sidecar container configuration
	// +kubebuilder:validation:Required
	Sidecar SidecarConfig `json:"sidecar"`

	// InitContainers defines optional init containers to inject alongside the sidecar
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Volumes defines additional volumes to mount in the pod
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`
}

// SidecarConfig defines the Vector sidecar container configuration
type SidecarConfig struct {
	// Name of the sidecar container
	// +kubebuilder:default=vector
	Name string `json:"name,omitempty"`

	// Image is the Vector container image
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9.\-/:]+:[a-zA-Z0-9.\-_]+$`
	Image string `json:"image"`

	// ImagePullPolicy for the sidecar container
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Config defines the Vector configuration source
	// +kubebuilder:validation:Required
	Config VectorConfig `json:"config"`

	// VolumeMounts defines volume mounts for the sidecar container
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Resources defines compute resource requirements
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Env defines environment variables for the sidecar
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Args defines additional arguments for the Vector binary
	// +optional
	Args []string `json:"args,omitempty"`
}

// VectorConfig defines the configuration source for Vector
type VectorConfig struct {
	// ConfigMapRef references a ConfigMap containing Vector configuration
	// +optional
	ConfigMapRef *ConfigMapRef `json:"configMapRef,omitempty"`

	// Inline contains inline Vector configuration (YAML or TOML)
	// +optional
	Inline string `json:"inline,omitempty"`
}

// ConfigMapRef references a ConfigMap key
type ConfigMapRef struct {
	// Name of the ConfigMap
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key in the ConfigMap containing the configuration
	// +kubebuilder:default=vector.yaml
	Key string `json:"key,omitempty"`
}

// VectorSidecarStatus defines the observed state of VectorSidecar
type VectorSidecarStatus struct {
	// Conditions represent the latest available observations of the VectorSidecar's state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// MatchedDeployments is the number of Deployments matching the selector
	// +optional
	MatchedDeployments int32 `json:"matchedDeployments,omitempty"`

	// InjectedDeployments is the number of Deployments with injected sidecars
	// +optional
	InjectedDeployments int32 `json:"injectedDeployments,omitempty"`

	// LastUpdateTime is the timestamp of the last status update
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// InjectedHash is the hash of the current injection configuration
	// +optional
	InjectedHash string `json:"injectedHash,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed VectorSidecar
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// Condition types for VectorSidecar
const (
	// ConditionTypeReady indicates the VectorSidecar is ready and injecting sidecars
	ConditionTypeReady string = "Ready"

	// ConditionTypeError indicates an error occurred during reconciliation
	ConditionTypeError string = "Error"

	// ConditionTypeConfigValid indicates the Vector configuration is valid
	ConditionTypeConfigValid string = "ConfigValid"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=vs
//+kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled"
//+kubebuilder:printcolumn:name="Matched",type="integer",JSONPath=".status.matchedDeployments"
//+kubebuilder:printcolumn:name="Injected",type="integer",JSONPath=".status.injectedDeployments"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// VectorSidecar is the Schema for the vectorsidecars API
type VectorSidecar struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VectorSidecarSpec   `json:"spec,omitempty"`
	Status VectorSidecarStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VectorSidecarList contains a list of VectorSidecar
type VectorSidecarList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VectorSidecar `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VectorSidecar{}, &VectorSidecarList{})
}
