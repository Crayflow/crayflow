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

package controllers

import (
	"context"
	"fmt"
	devopsv1 "github.com/buhuipao/crayflow/api/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const WorkflowFinalizerName = "delete_workflow"

// WorkflowReconciler reconciles a Workflow object
type WorkflowReconciler struct {
	client.Client
	Eventer record.EventRecorder
	Scheme  *runtime.Scheme
}

//+kubebuilder:rbac:groups=devops.crayflow.xyz,resources=workflows,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=devops.crayflow.xyz,resources=workflows/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=devops.crayflow.xyz,resources=workflows/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Workflow object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *WorkflowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// TODO(user): your logic here
	var workflow devopsv1.Workflow
	if err := r.Get(ctx, req.NamespacedName, &workflow); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "unable to get Workflow")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// OnDelete
	if !workflow.DeletionTimestamp.IsZero() {
		// workflow onDelete
		return r.ProcessOnDelete(ctx, &workflow)
	}

	// check whether the workflow has a finalizer flag
	if len(workflow.Finalizers) == 0 {
		workflow.Finalizers = append(workflow.Finalizers, WorkflowFinalizerName)
		if err := r.Update(ctx, &workflow); err != nil {
			return ctrl.Result{}, err
		}
	}

	if workflow.Status.Phase == "" {
		workflow.Status.Phase = devopsv1.DefaultWorkflowPhase
	}

	// update to 'running' status
	if workflow.Status.Phase == devopsv1.WorkflowPhasePending {
		workflow.Status.Phase = devopsv1.WorkflowPhaseRunning
	}

	// check cycle
	if CheckCycle(&workflow) {
		workflow.Status.Phase = devopsv1.WorkflowPhaseFailed
		workflow.Status.Reason, workflow.Status.Message = devopsv1.ErrWorkflowHasCycle.Error(), "workflow has cycle"
		if err := r.Update(ctx, &workflow); err != nil {
			logger.Error(err, "update workflow failed")
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}

	// OnUpdate
	// if .Spec.Resets field changed, and the controller will re-run nodes that are in .Spec.Resets field
	if len(workflow.Spec.Resets) != 0 {
		return r.ProcessReset(ctx, &workflow)
	}

	// workflow OnCreate or OnSchedule
	runningNodes := FindWorkflowRunningNodes(ctx, &workflow)
	// process running nodes
	if err := r.ProcessWorkflowRunningNodes(ctx, &workflow, runningNodes); err != nil {
		return ctrl.Result{}, err
	}

	readyNodes := FindWorkflowReadyNodes(ctx, &workflow)
	// trigger ready nodes
	if err := r.ProcessWorkflowReadyNodes(ctx, &workflow, readyNodes); err != nil {
		return ctrl.Result{}, err
	}

	// check node status then update workflow status
	for i := range workflow.Status.Nodes {
		node := workflow.Status.Nodes[i]
		if node.Phase == devopsv1.NodePhaseFailed || node.Phase == devopsv1.NodePhaseTimeout {
			workflow.Status.Phase = devopsv1.WorkflowPhaseFailed
		}
	}

	// update the workflow to end state
	if len(runningNodes) == 0 && len(readyNodes) == 0 && workflow.Status.Phase != devopsv1.WorkflowPhaseFailed {
		workflow.Status.Phase = devopsv1.WorkflowPhaseSuccess
	}
	if err := r.Update(ctx, &workflow); err != nil {
		logger.Error(err, fmt.Sprintf("update workflow failed"))
		return ctrl.Result{}, err
	}
	logger.Info(fmt.Sprintf("update workflow status succeeded"))

	return ctrl.Result{}, nil
}

func (r *WorkflowReconciler) ProcessOnDelete(ctx context.Context, workflow *devopsv1.Workflow) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if len(workflow.Finalizers) != 0 && workflow.Finalizers[0] == WorkflowFinalizerName {
		// record the finalizer evnet
		r.Eventer.Event(workflow, "delete", "some reason", "remove finalizer flag")

		// do something with the finalizers, such as delete the running resource of the workflow
		logger.Info("do something with the finalizers, and set the finalizers to empty")
		// do something ...

		workflow.Finalizers = workflow.Finalizers[:len(workflow.Finalizers)-1]
		if err := r.Update(ctx, workflow); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// workflow real deleted
	return ctrl.Result{}, nil
}

func (r *WorkflowReconciler) ProcessReset(ctx context.Context, workflow *devopsv1.Workflow) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	nodes := workflow.Spec.Resets

	if workflow.Spec.Clear {
		var err error
		nodes, err = FindPostNodes(workflow, nodes)
		if err != nil && err != devopsv1.ErrWorkflowHasCycle {
			logger.Error(err, "find workflow post nodes when clear", "resetNodes", workflow.Spec.Resets)
			return ctrl.Result{}, err
		}
		if err == devopsv1.ErrWorkflowHasCycle {
			logger.Error(err, "workflow has cycle, update to 'failed' status")
			workflow.Status.Phase = devopsv1.WorkflowPhaseFailed
			if err := r.Update(ctx, workflow); err != nil {
				logger.Error(err, "update workflow failed")
			}
			logger.Info("updated workflow to 'failed' status successfully")
			return ctrl.Result{}, nil
		}
	}

	// remove node status
	for i := range nodes {
		logger.Info("removing node status", "node", nodes[i])
		RemoveWorkflowNodeStatus(workflow, nodes[i])
	}
	workflow.Spec.Resets, workflow.Spec.Clear = nil, false
	workflow.Status.Phase = devopsv1.WorkflowPhaseRunning
	if err := r.Update(ctx, workflow); err != nil {
		logger.Error(err, "update workflow failed")
	}
	logger.Info("updated workflow to 'running' status successfully")

	return ctrl.Result{Requeue: true}, nil
}

func (r *WorkflowReconciler) ProcessWorkflowRunningNodes(ctx context.Context, workflow *devopsv1.Workflow,
	runningNodes []devopsv1.WorkflowNodeStatus) error {
	logger := log.FromContext(ctx)

	workflow.Status.RunningNodes = nil
	for i := range runningNodes {
		node, nodePhase := runningNodes[i], fmt.Sprintf("%s:%s", runningNodes[i].Name, runningNodes[i].Phase)
		workflow.Status.RunningNodes = append(workflow.Status.RunningNodes, nodePhase)
		var nodeSpec *devopsv1.WorkflowNodeSpec
		for i := range workflow.Spec.Nodes {
			if workflow.Spec.Nodes[i].Name == node.Name {
				nodeSpec = &workflow.Spec.Nodes[i]
				break
			}
		}
		if nodeSpec == nil {
			logger.Error(devopsv1.ErrNodeNotFound, "not found spec of running node", "node", node.Name)
			// clean this running node, remove node status
			RemoveWorkflowNodeStatus(workflow, node.Name)
			continue
		}

		var nodeStatus *devopsv1.WorkflowNodeStatus
		// if node's workload is empty, create workload for node
		if node.Workload == nil {
			logger.Info("'running' status node's workload is empty, creating workload for it", "node", node.Name)
			pod, err := r.createWorkloadForNode(ctx, workflow, node.Name)
			if err != nil {
				return err
			}
			nodeStatus = &devopsv1.WorkflowNodeStatus{
				Name:     node.Name,
				Phase:    devopsv1.NodePhaseRunning,
				Workload: pod,
			}
			UpdateWorkflowNodeStatus(workflow, nodeStatus)
			continue
		}

		// check workload status
		var pod v1.Pod
		err := r.Get(ctx, types.NamespacedName{Namespace: node.Workload.Namespace, Name: node.Workload.Name}, &pod)
		if err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "get node status pod failed", "node", node.Name)
			return err
		}
		if apierrors.IsNotFound(err) {
			logger.Info("workload of node has been deleted, creating for it", "node", node.Name)
			pod, err := r.createWorkloadForNode(ctx, workflow, node.Name)
			if err != nil {
				return err
			}
			nodeStatus = &devopsv1.WorkflowNodeStatus{
				Name:     node.Name,
				Phase:    devopsv1.NodePhaseRunning,
				Workload: pod,
			}
		} else {
			logger.Info("get workload of node successfully", "node", node.Name)
			nodeStatus = &devopsv1.WorkflowNodeStatus{
				Name:     node.Name,
				Workload: &pod,
			}
			switch pod.Status.Phase {
			case v1.PodPending, v1.PodRunning:
				nodeStatus.Phase = devopsv1.NodePhaseRunning
			case v1.PodFailed:
				nodeStatus.Phase = devopsv1.NodePhaseFailed
				nodeStatus.Reason = "pod failed"
			case v1.PodSucceeded:
				nodeStatus.Phase = devopsv1.NodePhaseSuccess
				nodeStatus.Reason = "pod succeeded"
			default:
				nodeStatus.Phase = devopsv1.NodePhaseRunning
			}
			// check timeout
			if nodeStatus.Phase == devopsv1.NodePhaseRunning &&
				pod.CreationTimestamp.Add(nodeSpec.Timeout*time.Second).Before(time.Now()) {
				nodeStatus.Phase = devopsv1.NodePhaseTimeout
			}
		}

		UpdateWorkflowNodeStatus(workflow, nodeStatus)
	}

	return nil
}

func (r *WorkflowReconciler) ProcessWorkflowReadyNodes(ctx context.Context, workflow *devopsv1.Workflow,
	readyNodes []devopsv1.WorkflowNodeSpec) error {
	logger := log.FromContext(ctx)
	for i := range readyNodes {
		node := readyNodes[i]
		pod, err := r.createWorkloadForNode(ctx, workflow, node.Name)
		if err != nil {
			return err
		}

		logger.Info("created pod succeeded", "node", node.Name)
		nodeStatus := &devopsv1.WorkflowNodeStatus{
			Name:     node.Name,
			Phase:    devopsv1.NodePhaseRunning,
			Message:  "created pod succeeded",
			Workload: pod,
		}

		logger.Info(fmt.Sprintf("update node status"), "node", node.Name, "status", nodeStatus)
		UpdateWorkflowNodeStatus(workflow, nodeStatus)

		// update the workflow status to running
		if workflow.Status.Phase != devopsv1.WorkflowPhaseFailed {
			workflow.Status.Phase = devopsv1.WorkflowPhaseRunning
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devopsv1.Workflow{}).
		Owns(&v1.Pod{}).
		Complete(r)
}

func (r *WorkflowReconciler) createWorkloadForNode(ctx context.Context, workflow *devopsv1.Workflow, nodeName string) (
	*v1.Pod, error) {
	logger := log.FromContext(ctx)

	var node *devopsv1.WorkflowNodeSpec
	for i := range workflow.Spec.Nodes {
		if workflow.Spec.Nodes[i].Name == nodeName {
			node = &workflow.Spec.Nodes[i]
			break
		}
	}
	if node == nil {
		logger.Error(devopsv1.ErrNodeNotFound, fmt.Sprintf("node %s not found", nodeName))
		return nil, devopsv1.ErrNodeNotFound
	}

	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("crayflow-%s-%s", workflow.Name, node.Name),
			Labels: map[string]string{
				"crayflow/workflow": workflow.Name,
				"crayflow/node":     node.Name,
			},
			Namespace: workflow.Namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{*node.Container},
		},
	}
	if err := controllerutil.SetOwnerReference(workflow, &pod, r.Scheme); err != nil {
		logger.Error(err, "set pod owner reference failed", "node", node.Name)
		return nil, err
	}

	if err := r.Create(ctx, &pod); err != nil {
		logger.Error(err, "create pod failed", "node", node.Name)
		return nil, err
	}

	return &pod, nil
}

/*
---------------------------------------------------------------- workflow logic --------------------------------
*/

func FindPostNodes(workflow *devopsv1.Workflow, curNodes []string) ([]string, error) {
	// check cycle
	if CheckCycle(workflow) {
		return nil, devopsv1.ErrWorkflowHasCycle
	}

	// build node outgoings
	nodeOutgoings := make(map[string][]string, len(workflow.Spec.Nodes))
	for i := range workflow.Spec.Nodes {
		node := workflow.Spec.Nodes[i]
		nodeOutgoings[node.Name] = make([]string, 0)
		for j := range node.Dependencies {
			if _, ok := nodeOutgoings[node.Dependencies[j]]; !ok {
				nodeOutgoings[node.Dependencies[j]] = make([]string, 0)
			}
			nodeOutgoings[node.Dependencies[j]] = append(nodeOutgoings[node.Dependencies[j]], node.Name)
		}
	}

	// visit the dag
	queue := curNodes
	visited := make(map[string]bool, len(curNodes))
	var node string
	var nodes []string
	for len(queue) != 0 {
		node, queue = queue[0], queue[1:]
		if !visited[node] {
			nodes = append(nodes, node)
			queue = append(queue, nodeOutgoings[node]...)
		}
		visited[node] = true
	}

	return nodes, nil
}

func FindWorkflowRunningNodes(ctx context.Context, workflow *devopsv1.Workflow) []devopsv1.WorkflowNodeStatus {
	var nodes []devopsv1.WorkflowNodeStatus
	for i := range workflow.Status.Nodes {
		node := workflow.Status.Nodes[i]
		if node.Phase == devopsv1.NodePhaseRunning {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func FindWorkflowReadyNodes(ctx context.Context, workflow *devopsv1.Workflow) []devopsv1.WorkflowNodeSpec {
	// build status mapping relationships
	node2Status := make(map[string]*devopsv1.WorkflowNodeStatus, len(workflow.Status.Nodes))
	for i := range workflow.Status.Nodes {
		node := workflow.Status.Nodes[i]
		node2Status[node.Name] = &node
	}

	// find ready nodes
	var nodes []devopsv1.WorkflowNodeSpec
	for i := range workflow.Spec.Nodes {
		node := workflow.Spec.Nodes[i]
		ready := true
		for j := range node.Dependencies {
			nodeStatus, ok := node2Status[node.Dependencies[j]]
			// Feature: here can support more strategies, but not now
			if !ok || nodeStatus.Phase != devopsv1.NodePhaseSuccess {
				ready = false
				break
			}
		}
		if ready {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func UpdateWorkflowNodeStatus(workflow *devopsv1.Workflow, nodeStatus *devopsv1.WorkflowNodeStatus) {
	if nodeStatus == nil {
		return
	}

	// replace the history node status or append new node status
	var updated bool
	for i := len(workflow.Status.Nodes) - 1; i >= 0; i-- {
		n := workflow.Status.Nodes[i]
		if n.Name == nodeStatus.Name {
			workflow.Status.Nodes[i] = *nodeStatus
			updated = true
			break
		}
	}

	if !updated {
		workflow.Status.Nodes = append(workflow.Status.Nodes, *nodeStatus)
	}
}

// RemoveWorkflowNodeStatus ...
func RemoveWorkflowNodeStatus(workflow *devopsv1.Workflow, nodeName string) {
	l, r := 0, len(workflow.Status.Nodes)-1
	for l <= r {
		for l <= r && workflow.Status.Nodes[l].Name == nodeName {
			workflow.Status.Nodes[l], workflow.Status.Nodes[r] = workflow.Status.Nodes[r], workflow.Status.Nodes[l]
			r--
		}
		if l > r {
			break
		}
		l++
	}

	workflow.Status.Nodes = workflow.Status.Nodes[:l]
}

// CheckCycle ...
func CheckCycle(workflow *devopsv1.Workflow) bool {
	// TODO: find node by dag

	return false
}
