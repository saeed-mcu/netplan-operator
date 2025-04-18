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

const (
	NoError    = "Done Successfully"
	Processing = "Processing"
	NotMatch   = "No matching nodes"
	NetplanErr = "Netplan Apply Failed"
)

const (
	ReasonCRNotAvailable          = "OperatorResourceNotAvailable"
	ReasonDeploymentNotAvailable  = "OperandDeploymentNotAvailable"
	ReasonOperandDeploymentFailed = "OperandDeploymentFailed"
	ReasonSucceeded               = "OperatorSucceeded"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NetplanConfigSpec defines the desired state of NetplanConfig
type NetplanConfigSpec struct {
	// NodeSelector is a selector which must be true for the policy to be applied to the node.
	// Selector which must match a node's labels for the policy to be scheduled on that node.
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// The desired configuration of the policy
	NetworkConfig string `json:"networkConfig,omitempty"`
}

// NetplanConfigStatus defines the observed state of NetplanConfig
type NetplanConfigStatus struct {
	// Conditions is the list of status condition updates
	Conditions []metav1.Condition `json:"conditions"`

	Applied string `json:"applied,omitempty"`
	State   string `json:"state,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Applied",type=string,JSONPath=`.status.applied`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

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
