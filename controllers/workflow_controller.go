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
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const WorkflowFinalizerName = "delete_workflow"
const (
	VariableConfigMapMountName = "crayflow-variable-config"
	VariableConfigMapMountPath = "/crayflow/workspace/vars"

	ToolsContainerName  = "tools"
	ToolsContainerImage = "buhuipao/crayflow-tools:latest"
	ToolsMountName      = "crayflow-tools"
	ToolsOriginPath     = "/tools"
	ToolsMountPath      = "/crayflow/tools/"

	ServiceAccountName = "crayflow-controller-manager"
)

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
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

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
	workflow := &devopsv1.Workflow{}
	if err := r.Get(ctx, req.NamespacedName, workflow); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Workflow was deleted")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// OnDelete
	if !workflow.DeletionTimestamp.IsZero() {
		// workflow onDelete
		return r.ProcessOnDelete(ctx, workflow)
	}

	// check whether the workflow has a finalizer flag
	if len(workflow.Finalizers) == 0 {
		workflow.Finalizers = append(workflow.Finalizers, WorkflowFinalizerName)
		if err := r.Update(ctx, workflow); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// TODO: create configmap for workflow as variable storage

	if workflow.Status.Phase == "" {
		r.Eventer.Event(workflow, v1.EventTypeNormal, "Initial", "Initial workflow phase, update to 'Pending'")
		workflow.Status.Phase, workflow.Status.Total = devopsv1.DefaultWorkflowPhase, len(workflow.Spec.Nodes)
		if err := r.Status().Update(ctx, workflow); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// update to 'running' status
	if workflow.Status.Phase == devopsv1.WorkflowPhasePending {
		r.Eventer.Event(workflow, v1.EventTypeNormal, "Schedule", "Update workflow phase to 'Running'")
		workflow.Status.Phase, workflow.Status.Total = devopsv1.WorkflowPhaseRunning, len(workflow.Spec.Nodes)
		if err := r.Status().Update(ctx, workflow); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// check cycle
	if workflow.CheckCycle() {
		r.Eventer.Event(workflow, v1.EventTypeWarning, "FoundCycle", "Update workflow phase to 'Failed'")
		workflow.Status.Phase = devopsv1.WorkflowPhaseFailed
		workflow.Status.Reason, workflow.Status.Message = devopsv1.ErrWorkflowHasCycle.Error(), "workflow has cycle"
		if err := r.Status().Update(ctx, workflow); err != nil {
			logger.Error(err, "update workflow failed")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// OnUpdate
	// if .Spec.Resets field changed, and the controller will re-run nodes that are in .Spec.Resets field
	if len(workflow.Spec.Resets) != 0 {
		return r.ProcessReset(ctx, workflow)
	}
	workflow.Status.Clear, workflow.Status.Resets = false, nil
	workflow.Status.Total = len(workflow.Spec.Nodes)

	// workflow OnCreate or OnSchedule
	runningNodes := workflow.FindWorkflowRunningNodes()
	// process running nodes
	if err := r.ProcessWorkflowRunningNodes(ctx, workflow, runningNodes); err != nil {
		return ctrl.Result{}, err
	}

	readyNodes := workflow.FindWorkflowReadyNodes()
	// trigger ready nodes
	if err := r.ProcessWorkflowReadyNodes(ctx, workflow, readyNodes); err != nil {
		return ctrl.Result{}, err
	}
	workflow.Status.RunningCount = len(runningNodes) + len(readyNodes)

	// check node status then update workflow status
	for i := range workflow.Status.Nodes {
		node := workflow.Status.Nodes[i]
		if node.Phase == devopsv1.NodePhaseFailed || node.Phase == devopsv1.NodePhaseTimeout {
			r.Eventer.Event(workflow, v1.EventTypeWarning, "NodeException", fmt.Sprintf("Node %s exception, update workflow to 'Failed'", node.Name))
			workflow.Status.Phase = devopsv1.WorkflowPhaseFailed
		}
	}

	// update the workflow to end state
	if len(runningNodes) == 0 && len(readyNodes) == 0 && workflow.Status.Phase == devopsv1.WorkflowPhaseRunning {
		r.Eventer.Event(workflow, v1.EventTypeNormal, "ScheduleFinish", "All nodes finish, update phase to 'Success'")
		workflow.Status.Phase = devopsv1.WorkflowPhaseSuccess
	}
	if err := r.Status().Update(ctx, workflow); err != nil {
		logger.Error(err, "update workflow failed in reconcile")
		return ctrl.Result{}, err
	}
	logger.Info("update workflow status succeeded in reconcile")

	return ctrl.Result{}, nil
}

func (r *WorkflowReconciler) ProcessOnDelete(ctx context.Context, workflow *devopsv1.Workflow) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if len(workflow.Finalizers) != 0 && workflow.Finalizers[0] == WorkflowFinalizerName {
		// record the finalizer event
		r.Eventer.Event(workflow, "delete", "some reason", "remove finalizer flag")

		// do something with the finalizers, such as delete the running resource of the workflow
		logger.Info("delete workloads of nodes, then set the finalizers to empty")
		// delete workload of nodes
		nodes := append(workflow.Status.HistoryNodes, workflow.Status.Nodes...)
		for i := range nodes {
			if nodes[i].Workload != nil {
				obj := v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nodes[i].Workload.Name,
						Namespace: nodes[i].Workload.Namespace,
					},
				}
				logger.Info("delete workload of node", "node", nodes[i].Name, "workload", obj)
				_ = r.Delete(ctx, &obj)
			}
		}

		workflow.Finalizers = workflow.Finalizers[:len(workflow.Finalizers)-1]
		if err := r.Update(ctx, workflow); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// workflow real deleted
	return ctrl.Result{}, nil
}

func (r *WorkflowReconciler) ProcessReset(ctx context.Context, workflow *devopsv1.Workflow) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var changed bool
	if !reflect.DeepEqual(workflow.Status.Resets, workflow.Spec.Resets) {
		logger.Info("'resets' has changed", "old", workflow.Status.Resets, "new", workflow.Spec.Resets)
		changed = true
	}
	if workflow.Status.Clear != workflow.Spec.Clear {
		logger.Info("'clear' has changed", "old", workflow.Status.Clear, "new", workflow.Spec.Clear)
		changed = true
	}
	// remove node status and change workflow status
	// copy 'resets' and 'clear' to status
	if changed {
		nodes := workflow.Spec.Resets
		if workflow.Spec.Clear {
			nodes = workflow.FindPostNodes(nodes)
		}
		for i := range nodes {
			logger.Info("removing node status", "node", nodes[i])
			workflow.RemoveWorkflowNodeStatus(nodes[i])
		}
		workflow.Status.Total = len(workflow.Spec.Nodes)
		workflow.Status.Phase = devopsv1.WorkflowPhaseRunning
		workflow.Status.Resets, workflow.Status.Clear = workflow.Spec.Resets, workflow.Spec.Clear
		r.Eventer.Event(workflow, v1.EventTypeNormal, "Reset", "Update phase to 'Running'")
		if err := r.Status().Update(ctx, workflow); err != nil {
			logger.Error(err, "update workflow failed in process reset")
			return ctrl.Result{}, nil
		}
		logger.Info("updated workflow to 'running' status successfully in process reset")
		return ctrl.Result{}, nil
	}

	// clean resets and clear flag
	workflow.Spec.Resets, workflow.Spec.Clear = nil, false
	r.Eventer.Event(workflow, v1.EventTypeNormal, "Reset", "Clean 'Resets'")
	if err := r.Update(ctx, workflow); err != nil {
		logger.Error(err, "clean workflow resets failed")
		return ctrl.Result{}, nil
	}
	logger.Info("clean workflow resets successfully")
	return ctrl.Result{}, nil
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
			workflow.RemoveWorkflowNodeStatus(node.Name)
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
			nodeStatus = GenerateNodeStatusWithWorkload(node.Name, pod)
			workflow.UpdateWorkflowNodeStatus(nodeStatus)
			continue
		}

		// check workload status
		var pod v1.Pod
		var err error
		logger.Info("getting workload of node", "node", node.Name, "workload", node.Workload.Name)
		if err = r.Get(ctx, types.NamespacedName{Namespace: node.Workload.Namespace, Name: node.Workload.Name},
			&pod); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "get workload of node failed", "node", node.Name)
			return err
		}
		if apierrors.IsNotFound(err) {
			logger.Info("workload of node has been deleted, creating workload for it",
				"node", node.Name, "workload", node.Workload.Name)
			pod, err := r.createWorkloadForNode(ctx, workflow, node.Name)
			if err != nil {
				return err
			}
			nodeStatus = GenerateNodeStatusWithWorkload(node.Name, pod)
		} else {
			logger.Info("get workload of node successfully", "node", node.Name,
				"workload", node.Workload.Name, "workload.Phase", pod.Status.Phase)
			nodeStatus = GenerateNodeStatusWithWorkload(node.Name, &pod)
			switch pod.Status.Phase {
			case v1.PodPending, v1.PodRunning:
				nodeStatus.Phase = devopsv1.NodePhaseRunning
			case v1.PodFailed:
				r.Eventer.Event(workflow, v1.EventTypeWarning, "WorkloadFailed", fmt.Sprintf("Update node %s to 'Failed'", node.Name))
				nodeStatus.Phase = devopsv1.NodePhaseFailed
				nodeStatus.Reason, nodeStatus.EndTime = "pod failed", time.Now().String()
			case v1.PodSucceeded:
				r.Eventer.Event(workflow, v1.EventTypeNormal, "WorkloadSuccess", fmt.Sprintf("Update node %s to 'Success'", node.Name))
				nodeStatus.Phase = devopsv1.NodePhaseSuccess
				nodeStatus.Reason, nodeStatus.EndTime = "pod succeeded", time.Now().String()
			default:
				nodeStatus.Phase = devopsv1.NodePhaseRunning
			}
			// check timeout
			if nodeStatus.Phase == devopsv1.NodePhaseRunning &&
				pod.CreationTimestamp.Add(nodeSpec.Timeout*time.Second).Before(time.Now()) {
				logger.Error(devopsv1.ErrNodeTimeout, "workload of node timeout")
				r.Eventer.Event(workflow, v1.EventTypeWarning, "WorkloadTimeout", fmt.Sprintf("Update node %s to 'Timeout'", node.Name))
				nodeStatus.Phase, nodeStatus.EndTime = devopsv1.NodePhaseTimeout, time.Now().String()
			}
		}

		workflow.UpdateWorkflowNodeStatus(nodeStatus)
	}

	return nil
}

func (r *WorkflowReconciler) ProcessWorkflowReadyNodes(ctx context.Context, workflow *devopsv1.Workflow,
	readyNodes []devopsv1.WorkflowNodeSpec) error {
	logger := log.FromContext(ctx)
	for i := range readyNodes {
		node := readyNodes[i]
		logger.Info("creating workload for ready node", "node", node.Name)
		pod, err := r.createWorkloadForNode(ctx, workflow, node.Name)
		if err != nil {
			logger.Error(err, "created workload of node failed", "node", node.Name)
			return err
		}

		logger.Info("created workload succeeded", "node", node.Name, "workload", pod.Name)
		nodeStatus := GenerateNodeStatusWithWorkload(node.Name, pod)
		logger.Info(fmt.Sprintf("update node status"), "node", node.Name,
			"status", nodeStatus.Phase)
		workflow.UpdateWorkflowNodeStatus(nodeStatus)
		workflow.Status.RunningNodes = append(workflow.Status.RunningNodes,
			fmt.Sprintf("%s:%s", node.Name, devopsv1.NodePhaseRunning),
		)

		r.Eventer.Event(workflow, v1.EventTypeNormal, "ScheduleNode", fmt.Sprintf("Schedule node %s", node.Name))
		workflow.Status.Phase = devopsv1.WorkflowPhaseRunning
	}

	return nil
}

// GenerateNodeStatusWithWorkload ...
func GenerateNodeStatusWithWorkload(nodeName string, pod *v1.Pod) *devopsv1.WorkflowNodeStatus {
	return &devopsv1.WorkflowNodeStatus{
		Name:      nodeName,
		Phase:     devopsv1.NodePhaseRunning,
		Message:   "created pod succeeded",
		StartTime: time.Now().String(),
		Workload: &devopsv1.NodeWorkload{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Phase:     pod.Status.Phase,
			Message:   pod.Status.Message,
			Reason:    pod.Status.Reason,
		},
	}
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

	if node.Container.Name == "" {
		node.Container.Name = "workload"
	}
	toolsVolume := v1.Volume{
		Name: ToolsMountName,
		VolumeSource: v1.VolumeSource{
			EmptyDir: nil,
		},
	}
	toolsMount := v1.VolumeMount{
		Name:      ToolsMountName,
		MountPath: ToolsMountPath,
	}
	toolsContainer := v1.Container{
		Name:         ToolsContainerName,
		Image:        ToolsContainerImage,
		VolumeMounts: []v1.VolumeMount{toolsMount},
		Command:      []string{"sh"},
		Args: []string{
			"-c",
			fmt.Sprintf("ls -al %s && cp %s/* %s", ToolsOriginPath, ToolsOriginPath, ToolsMountPath),
		},
	}

	node.Container.VolumeMounts = append(node.Container.VolumeMounts, v1.VolumeMount{
		Name:      ToolsMountName,
		MountPath: ToolsMountPath,
	})
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("crayflow-%s-%s-", workflow.Name, node.Name),
			Labels: map[string]string{
				"crayflow/workflow": workflow.Name,
				"crayflow/node":     node.Name,
			},
			Namespace: workflow.Namespace,
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				toolsContainer,
			},
			Containers: []v1.Container{
				*node.Container,
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				toolsVolume,
			},
			// TODO: serviceAccount for get or update configmaps
		},
	}
	if err := controllerutil.SetOwnerReference(workflow, &pod, r.Scheme); err != nil {
		logger.Error(err, "set pod owner reference failed", "node", node.Name)
		return nil, err
	}

	logger.Info("creating workload for node", "node", node.Name)
	r.Eventer.Event(workflow, v1.EventTypeNormal, "CreateWorkload", fmt.Sprintf("Create workload for node %s", node.Name))
	if err := r.Create(ctx, &pod); err != nil {
		logger.Error(err, "create workload for node failed", "node", node.Name)
		return nil, err
	}

	logger.Info("created workload for node successfully", "node", node.Name, "workload", pod.Name)

	return &pod, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devopsv1.Workflow{}).
		Owns(&v1.Pod{}).
		/*
				WithEventFilter(predicate.Funcs{
					CreateFunc: nil,
					DeleteFunc: nil,
					UpdateFunc: func(event event.UpdateEvent) bool {
						o1, ok1 := event.ObjectOld.(*v1.Pod)
						o2, ok2 := event.ObjectNew.(*v1.Pod)
						if ok1 && ok2 {
							return o1.Status.Phase == o2.Status.Phase
						}
						return true
					},
					GenericFunc: nil,
				}).
			Watches(
				&source.Kind{Type: &v1.ConfigMap{}},
				handler.EnqueueRequestsFromMapFunc(r.findObjectsForConfigMap),
				builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
			).
		*/
		Complete(r)
}
