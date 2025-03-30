/*
Copyright 2025.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NetplanConfigSpec defines the desired state of NetplanConfig
type NetplanConfigSpec struct {
	// NetworkConfig is sample netplan config
	NetworkConfig string `json:"networkConfig,omitempty"`
}

// NetplanConfigStatus defines the observed state of NetplanConfig
type NetplanConfigStatus struct {
	Applied bool   `json:"applied,omitempty"`
	Error   string `json:"error,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NetplanConfig is the Schema for the netplanconfigs API
type NetplanConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetplanConfigSpec   `json:"spec,omitempty"`
	Status NetplanConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetplanConfigList contains a list of NetplanConfig
type NetplanConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetplanConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetplanConfig{}, &NetplanConfigList{})
}
