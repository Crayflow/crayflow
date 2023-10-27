/*
Copyright 2023.

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
	"errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"time"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WorkflowSpec defines the desired state of Workflow
type WorkflowSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Workflow. Edit workflow_types.go to remove/update
	// Foo string `json:"foo,omitempty"`

	Creator string `json:"creator,omitempty"`
	// +optional
	Description string             `json:"description,omitempty"`
	Nodes       []WorkflowNodeSpec `json:"nodes,omitempty"`
	// +optional
	Vars []WorkflowVariableSpec `json:"variables,omitempty"`
	// if user wants to restart the workflow, can set this field to name of some nodes,
	// controller will run the workflow from those nodes, after process,
	// controller will reset this field to be empty
	// +optional
	Resets []string `json:"resets,omitempty"`
	// +optional
	Clear bool `json:"clear,omitempty"`
}

type WorkflowVariableSpec struct {
	Key   string               `json:"key,omitempty"`
	Value runtime.RawExtension `json:"value,omitempty"`
}

type WorkflowNodeSpec struct {
	Name         string        `json:"name,omitempty"`
	Dependencies []string      `json:"dependencies,omitempty"`
	Timeout      time.Duration `json:"timeout,omitempty"`
	// Plugin    runtime.RawExtension `json:"plugin,omitempty"`
	Container *v1.Container `json:"container,omitempty"`
}

// WorkflowPhase is the workflow phase
// +enum
type WorkflowPhase string

const (
	WorkflowPhasePending WorkflowPhase = "Pending"
	WorkflowPhaseRunning WorkflowPhase = "Running"
	WorkflowPhaseFailed  WorkflowPhase = "Failed"
	WorkflowPhaseSuccess WorkflowPhase = "Success"
)

type NodePhase string

const (
	NodePhaseRunning NodePhase = "Running"
	NodePhaseFailed  NodePhase = "Failed"
	NodePhaseTimeout NodePhase = "Timeout"
	NodePhaseSuccess NodePhase = "Success"
)

var (
	ErrWorkflowHasCycle = errors.New("workflow has cycle")
	ErrNodeNotFound     = errors.New("node not found")
)

type WorkflowNodeStatus struct {
	Name     string    `json:"name,omitempty"`
	Phase    NodePhase `json:"phase,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	Message  string    `json:"message,omitempty"`
	Workload *v1.Pod   `json:"container,omitempty"`
	// Plugin    runtime.RawExtension `json:"plugin,omitempty"`
}

// WorkflowStatus defines the observed state of Workflow
type WorkflowStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase        WorkflowPhase        `json:"phase,omitempty"`
	Reason       string               `json:"reason,omitempty"`
	Message      string               `json:"message,omitempty"`
	RunningNodes []string             `json:"runningNodes,omitempty"`
	Nodes        []WorkflowNodeStatus `json:"nodes,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".status.phase",name=Phase,type=string
//+kubebuilder:printcolumn:JSONPath=".status.runningNodes",name=RunningNodes,type=string

// Workflow is the Schema for the workflows API
type Workflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkflowSpec   `json:"spec,omitempty"`
	Status WorkflowStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkflowList contains a list of Workflow
type WorkflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workflow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workflow{}, &WorkflowList{})
}
