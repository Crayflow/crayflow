package v1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestWorkflow_CheckRepeatName(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       WorkflowSpec
		Status     WorkflowStatus
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
		{
			name: "test duplication node names",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name: "test",
						},
						{
							Name: "test",
						},
					},
				},
			},
			want: true,
		},

		{
			name: "test normal workflow node specific",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name: "step1",
						},
						{
							Name: "step2",
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := &Workflow{
				Spec: tt.fields.Spec,
			}
			if got := workflow.CheckRepeatName(); got != tt.want {
				t.Errorf("CheckRepeatName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkflow_CheckInvalidDependencies(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       WorkflowSpec
		Status     WorkflowStatus
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
		{
			name: "test invalid dependencies",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name:         "step1",
							Dependencies: nil,
							Timeout:      0,
							Container:    nil,
						},
						{
							Name: "step2",
							Dependencies: []string{
								"step1",
								"invalid-step",
							},
							Timeout:   0,
							Container: nil,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test normal dependencies",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name:         "step0",
							Dependencies: nil,
							Timeout:      0,
							Container:    nil,
						},
						{
							Name:         "step1",
							Dependencies: nil,
							Timeout:      0,
							Container:    nil,
						},
						{
							Name: "step2",
							Dependencies: []string{
								"step1",
							},
							Timeout:   0,
							Container: nil,
						},
						{
							Name: "step3",
							Dependencies: []string{
								"step1",
								"step2",
							},
							Timeout:   0,
							Container: nil,
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := &Workflow{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
				Status:     tt.fields.Status,
			}
			if got := workflow.CheckInvalidDependencies(); got != tt.want {
				t.Errorf("CheckInvalidDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkflow_CheckEmpty(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       WorkflowSpec
		Status     WorkflowStatus
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
		{
			name: "empty workflow",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: nil,
				},
			},
			want: true,
		},
		{
			name: "normal workflow",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name: "step1",
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := &Workflow{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
				Status:     tt.fields.Status,
			}
			if got := workflow.CheckEmpty(); got != tt.want {
				t.Errorf("CheckEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkflow_CheckCycle(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       WorkflowSpec
		Status     WorkflowStatus
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
		{
			name: "self cycle workflow",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name: "test1",
							Dependencies: []string{
								"test1",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "two node cycle workflow",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name: "test1",
							Dependencies: []string{
								"test2",
							},
						},
						{
							Name: "test2",
							Dependencies: []string{
								"test1",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "link and cycle workflow",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name: "test1",
							Dependencies: []string{
								"test2",
							},
						},
						{
							Name: "test2",
							Dependencies: []string{
								"test1",
								"test3",
							},
						},
						{
							Name: "test3",
							Dependencies: []string{
								"test2",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "two cycle workflow",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name: "test1",
							Dependencies: []string{
								"test2",
								"test3",
							},
						},
						{
							Name: "test2",
							Dependencies: []string{
								"test1",
							},
						},
						{
							Name: "test3",
							Dependencies: []string{
								"test1",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "no cycle workflow",
			fields: fields{
				Spec: WorkflowSpec{
					Nodes: []WorkflowNodeSpec{
						{
							Name:         "test0",
							Dependencies: []string{},
						},
						{
							Name:         "test1",
							Dependencies: []string{},
						},
						{
							Name: "test2",
							Dependencies: []string{
								"test1",
							},
						},
						{
							Name: "test3",
							Dependencies: []string{
								"test2",
							},
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := &Workflow{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
				Status:     tt.fields.Status,
			}
			if got := workflow.CheckCycle(); got != tt.want {
				t.Errorf("CheckCycle() = %v, want %v", got, tt.want)
			}
		})
	}
}
