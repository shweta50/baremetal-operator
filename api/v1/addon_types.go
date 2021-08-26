/*


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
	"github.com/platform9/pf9-qbert/sunpike/apiserver/pkg/apis/sunpike/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AddonSpec defines the desired state of Addon
type AddonSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ClusterID string   `json:"clusterID"`
	Version   string   `json:"version"`
	Type      string   `json:"type"`
	Override  Override `json:"override,omitempty"`
	Watch     bool     `json:"watch,omitempty"`
	Retry     int32    `json:"retry,omitempty"`
}

// Override defines params to override in the addon
type Override struct {
	Params []Params `json:"params,omitempty"`
}

// Params defines params to override in the addon
type Params struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AddonStatus defines the observed state of Addon
type AddonStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ObservedGeneration int64               `json:"observedGeneration,omitempty"`
	Phase              v1alpha2.AddonPhase `json:"phase,omitempty"`
	Message            string              `json:"message,omitempty"`
	Healthy            bool                `json:"healthy,omitempty"`
}

// Addon is the Schema for the addons API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonSpec   `json:"spec,omitempty"`
	Status AddonStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AddonList contains a list of Addon
type AddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Addon{}, &AddonList{})
}