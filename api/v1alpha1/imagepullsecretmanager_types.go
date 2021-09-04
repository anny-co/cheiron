/*
Copyright 2021.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ImagePullSecretManagerSpec defines the desired state of ImagePullSecretManager
type ImagePullSecretManagerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Secrets is the list of ImagePullSecrets to attach to a service account
	Secrets []ImagePullSecretSpec `json:"secrets"`

	// +kubebuilder:default=ServiceAccount

	// Mode defines whether the controller reconciles pods or service accounts for imagePullSecrets
	Mode ReconciliationMode `json:"mode"`
}

// ImagePullSecretManagerStatus defines the observed state of ImagePullSecretManager
type ImagePullSecretManagerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespace

// ImagePullSecretManager is the Schema for the imagepullsecretmanagers API
type ImagePullSecretManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImagePullSecretManagerSpec   `json:"spec,omitempty"`
	Status ImagePullSecretManagerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ImagePullSecretManagerList contains a list of ImagePullSecretManager
type ImagePullSecretManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ImagePullSecretManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ImagePullSecretManager{}, &ImagePullSecretManagerList{})
}
