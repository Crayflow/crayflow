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

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-devops-crayflow-xyz-v1-workflow,mutating=false,failurePolicy=fail,sideEffects=None,groups=devops.crayflow.xyz,resources=workflows,verbs=create;update,versions=v1,name=vworkflow.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Workflow{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Workflow) ValidateCreate() error {
	workflowlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	if len(r.Spec.Nodes) == 0 {
		return ErrNodesMustBeSpecified
	}

	if r.CheckInComing() {
		return ErrWorkflowMissingNode
	}

	if r.CheckCycle() {
		return ErrWorkflowHasCycle
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Workflow) ValidateUpdate(old runtime.Object) error {
	workflowlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	if len(r.Spec.Nodes) == 0 {
		return ErrNodesMustBeSpecified
	}

	if r.CheckInComing() {
		return ErrWorkflowMissingNode
	}

	if r.CheckCycle() {
		return ErrWorkflowHasCycle
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Workflow) ValidateDelete() error {
	workflowlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
