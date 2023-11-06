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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var workflowlog = logf.Log.WithName("workflow-resource")

func (r *Workflow) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-devops-crayflow-xyz-v1-workflow,mutating=true,failurePolicy=fail,sideEffects=None,groups=devops.crayflow.xyz,resources=workflows,verbs=create;update,versions=v1,name=mworkflow.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Workflow{}

const (
	DefaultNodeTimeout   = 600
	DefaultCreator       = "default"
	DefaultWorkflowPhase = WorkflowPhasePending
)

var (
	ErrNodesMustBeSpecified = errors.New("nodes must be specified")
)

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Workflow) Default() {
	workflowlog.Info("default", "name", r.Name)
	// TODO(user): fill in your defaulting logic.

	if r.Spec.Creator == "" {
		r.Spec.Creator = DefaultCreator
	}

	if r.Status.Phase == "" {
		r.Status.Phase = DefaultWorkflowPhase
	}

	r.Status.Total = len(r.Spec.Nodes)

	for i := range r.Spec.Nodes {
		if r.Spec.Nodes[i].Timeout == 0 {
			r.Spec.Nodes[i].Timeout = DefaultNodeTimeout
		}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-devops-crayflow-xyz-v1-workflow,mutating=false,failurePolicy=fail,sideEffects=None,groups=devops.crayflow.xyz,resources=workflows,verbs=create;update,versions=v1,name=vworkflow.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Workflow{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Workflow) ValidateCreate() error {
	workflowlog.Info("validate create", "name", r.Name)
	workflowlog.Info("validate create", "spec", r.Spec)

	// TODO(user): fill in your validation logic upon object creation.

	return r.Check()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Workflow) ValidateUpdate(old runtime.Object) error {
	workflowlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.

	return r.Check()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Workflow) ValidateDelete() error {
	workflowlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

// Check ...
func (workflow *Workflow) Check() error {
	if workflow.CheckEmpty() {
		return ErrNodesMustBeSpecified
	}

	if workflow.CheckRepeatName() {
		return ErrHasRepeatNode
	}

	if workflow.CheckInvalidDependencies() {
		return ErrWorkflowMissingNode
	}

	if workflow.CheckCycle() {
		return ErrWorkflowHasCycle
	}

	return nil
}

// CheckEmpty ...
func (workflow *Workflow) CheckEmpty() bool {
	return len(workflow.Spec.Nodes) == 0
}

// CheckCycle ...
func (workflow *Workflow) CheckCycle() bool {
	// build in-degree and out-degree
	inDegree, outDegree := make(map[string]int), make(map[string][]string)
	for i := range workflow.Spec.Nodes {
		node := workflow.Spec.Nodes[i]
		inDegree[node.Name] += len(node.Dependencies)
		for j := range node.Dependencies {
			outDegree[node.Dependencies[j]] = append(outDegree[node.Dependencies[j]], node.Name)
		}
	}

	var queue []string
	// find zero in-degree nodes in dag
	for node, count := range inDegree {
		if count == 0 {
			queue = append(queue, node)
		}
	}
	// visit the dag
	var cur string
	var visited int
	for len(queue) != 0 {
		queue, cur = queue[1:], queue[0]
		for i := range outDegree[cur] {
			out := outDegree[cur][i]
			inDegree[out] -= 1
			if inDegree[out] == 0 {
				queue = append(queue, out)
			}
		}
		visited++
	}

	return visited != len(workflow.Spec.Nodes)
}

// CheckInvalidDependencies ...
func (workflow *Workflow) CheckInvalidDependencies() bool {
	// build node specific set
	nodeSet := make(map[string]bool, len(workflow.Spec.Nodes))
	for i := range workflow.Spec.Nodes {
		nodeSet[workflow.Spec.Nodes[i].Name] = true
	}

	// check dependencies
	for i := range workflow.Spec.Nodes {
		nodeDependencies := workflow.Spec.Nodes[i].Dependencies
		for j := range nodeDependencies {
			if !nodeSet[nodeDependencies[j]] {
				return true
			}
		}
	}

	return false
}

// CheckRepeatName ...
func (workflow *Workflow) CheckRepeatName() bool {
	nodeSet := make(map[string]bool, len(workflow.Spec.Nodes))
	for i := range workflow.Spec.Nodes {
		if _, ok := nodeSet[workflow.Spec.Nodes[i].Name]; ok {
			return true
		}
		nodeSet[workflow.Spec.Nodes[i].Name] = true
	}

	return false
}
