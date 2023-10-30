package v1

/*
---------------------------------------------------------------- workflow schedule logic --------------------------------
*/

func (workflow *Workflow) FindPostNodes(curNodes []string) []string {
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

	return nodes
}

// FindWorkflowRunningNodes ...
func (workflow *Workflow) FindWorkflowRunningNodes() []WorkflowNodeStatus {
	var nodes []WorkflowNodeStatus
	for i := range workflow.Status.Nodes {
		node := workflow.Status.Nodes[i]
		if node.Phase == NodePhaseRunning {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

// FindWorkflowReadyNodes ...
func (workflow *Workflow) FindWorkflowReadyNodes() []WorkflowNodeSpec {
	// build status mapping relationships
	node2Status := make(map[string]*WorkflowNodeStatus, len(workflow.Status.Nodes))
	for i := range workflow.Status.Nodes {
		node := workflow.Status.Nodes[i]
		node2Status[node.Name] = &node
	}

	// find ready nodes
	var nodes []WorkflowNodeSpec
	for i := range workflow.Spec.Nodes {
		node := workflow.Spec.Nodes[i]
		if _, ok := node2Status[node.Name]; ok {
			continue
		}

		ready := true
		for j := range node.Dependencies {
			nodeStatus, ok := node2Status[node.Dependencies[j]]
			// Feature: here can support more strategies, but not now
			if !ok || nodeStatus.Phase != NodePhaseSuccess {
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

func (workflow *Workflow) UpdateWorkflowNodeStatus(nodeStatus *WorkflowNodeStatus) {
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

	// remove the history nodes
	node2Spec := make(map[string]bool, len(workflow.Spec.Nodes))
	for i := range workflow.Spec.Nodes {
		node := workflow.Spec.Nodes[i]
		node2Spec[node.Name] = true
	}
	var newStatuses []WorkflowNodeStatus
	for i := range workflow.Status.Nodes {
		node := workflow.Status.Nodes[i]
		if node2Spec[node.Name] {
			newStatuses = append(newStatuses, node)
		} else {
			workflow.Status.HistoryNodes = append(workflow.Status.HistoryNodes, node)
		}
	}
	workflow.Status.Nodes = newStatuses
}

// RemoveWorkflowNodeStatus ...
func (workflow *Workflow) RemoveWorkflowNodeStatus(nodeName string) {
	l, r := 0, len(workflow.Status.Nodes)-1
	for l <= r {
		for l <= r && workflow.Status.Nodes[l].Name == nodeName {
			workflow.Status.Nodes[l], workflow.Status.Nodes[r] = workflow.Status.Nodes[r], workflow.Status.Nodes[l]
			workflow.Status.HistoryNodes = append(workflow.Status.HistoryNodes, workflow.Status.Nodes[r])
			r--
		}
		if l > r {
			break
		}
		l++
	}

	workflow.Status.Nodes = workflow.Status.Nodes[:l]
}
