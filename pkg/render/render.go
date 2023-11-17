package render

import (
	"bytes"
	devopsv1 "github.com/buhuipao/crayflow/api/v1"
	"strings"
	"text/template"
)

const (
	defaultName     = "render"
	convertLeftTag  = `"|{{`
	convertRightTag = `}}|"`
)

// Render ...
type Render interface {
	Do(tpl string) (string, error)
}

// instanceRender ...
type instanceRender struct {
	Status       string
	ErrorMessage string
	Vars         map[string]string
	Nodes        map[string]Node
}

// Node ...
type Node struct {
	Status       string
	ErrorMessage string
}

// New ...
func New(workflow *devopsv1.Workflow) Render {
	// vars
	envs, vars := make(map[string]string), make(map[string]string)
	for i := range workflow.Status.Vars {
		v := workflow.Status.Vars[i]
		envs[v.Key] = v.Value
	}

	// nodes
	nodes := make(map[string]Node, len(workflow.Status.Nodes))
	for i := range workflow.Status.Nodes {
		node := workflow.Status.Nodes[i]
		nodes[node.Name] = Node{
			Status:       string(node.Phase),
			ErrorMessage: node.Message,
		}
	}

	return instanceRender{
		Vars:         vars,
		Nodes:        nodes,
		Status:       string(workflow.Status.Phase),
		ErrorMessage: workflow.Status.Message,
	}
}

// Do ...
func (r instanceRender) Do(tpl string) (string, error) {
	// convert special tag
	s := strings.ReplaceAll(strings.ReplaceAll(tpl, convertLeftTag, "{{"), convertRightTag, "}}")

	// render
	t, err := template.New(defaultName).Parse(s)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if err := t.Execute(buf, r); err != nil {
		return "", err
	}

	return buf.String(), nil
}
