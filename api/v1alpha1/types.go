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
	corev1 "k8s.io/api/core/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReconciliationMode defines what resource the controller reconciles
type ReconciliationMode string

const (
	// In pod mode, the operator directly adds the attached image pull secrets to the PodSpec
	PodMode ReconciliationMode = "Pod"
	// in service account mode, the operator adds the attached image pull secrets to the ServiceAccount
	ServiceAccountMode ReconciliationMode = "ServiceAccount"
)

// ImagePullSecretSpec encodes a singular ImagePullSecret, either using existing secrets, or by providing the credentials explicitly
type ImagePullSecretSpec struct {
	// ExistingSecretRef is a local object reference to an existing kubernetes.io/dockerconfigjson secret object
	ExistingSecretRef corev1.LocalObjectReference `json:"existingSecretRef,omitempty"`
	// Registy hostname is the container registry to target
	Registry string `json:"registry,omitempty"`
	// Username is the plaintext username field for the credentials of the registry
	Username string `json:"username,omitempty"`
	// Password is the plaintext field for the password of the credentials for the registry
	Password string `json:"password,omitempty"`
	// Email encodes the credentials email address (required by at least hub.docker.io)
	Email string `json:"email,omitempty"`
	// Name of the container registry and secret name
	Name string `json:"name"`
}
