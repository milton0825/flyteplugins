package flytek8s

import (
	"context"
	"reflect"
	"testing"

	"github.com/lyft/flytestdlib/contextutils"

	"github.com/lyft/flyteidl/gen/pb-go/flyteidl/core"
	"github.com/stretchr/testify/assert"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	pluginsCore "github.com/lyft/flyteplugins/go/tasks/pluginmachinery/core"
)

func TestGetExecutionEnvVars(t *testing.T) {
	mock := mockTaskExecutionIdentifier{}
	envVars := GetExecutionEnvVars(mock)
	assert.Len(t, envVars, 11)
}

func TestGetTolerationsForResources(t *testing.T) {
	var empty []v12.Toleration
	var emptyConfig map[v12.ResourceName][]v12.Toleration

	tolGPU := v12.Toleration{
		Key:      "flyte/gpu",
		Value:    "dedicated",
		Operator: v12.TolerationOpEqual,
		Effect:   v12.TaintEffectNoSchedule,
	}

	tolStorage := v12.Toleration{
		Key:      "storage",
		Value:    "dedicated",
		Operator: v12.TolerationOpExists,
		Effect:   v12.TaintEffectNoSchedule,
	}

	type args struct {
		resources v12.ResourceRequirements
	}
	tests := []struct {
		name   string
		args   args
		setVal map[v12.ResourceName][]v12.Toleration
		want   []v12.Toleration
	}{
		{
			"no-tolerations-limits",
			args{
				v12.ResourceRequirements{
					Limits: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
					},
				},
			},
			emptyConfig,
			empty,
		},
		{
			"no-tolerations-req",
			args{
				v12.ResourceRequirements{
					Requests: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
					},
				},
			},
			emptyConfig,
			empty,
		},
		{
			"no-tolerations-both",
			args{
				v12.ResourceRequirements{
					Limits: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
					},
					Requests: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
					},
				},
			},
			emptyConfig,
			empty,
		},
		{
			"tolerations-limits",
			args{
				v12.ResourceRequirements{
					Limits: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
					},
				},
			},
			map[v12.ResourceName][]v12.Toleration{
				v12.ResourceStorage: {tolStorage},
				ResourceNvidiaGPU:   {tolGPU},
			},
			[]v12.Toleration{tolStorage},
		},
		{
			"tolerations-req",
			args{
				v12.ResourceRequirements{
					Requests: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
					},
				},
			},
			map[v12.ResourceName][]v12.Toleration{
				v12.ResourceStorage: {tolStorage},
				ResourceNvidiaGPU:   {tolGPU},
			},
			[]v12.Toleration{tolStorage},
		},
		{
			"tolerations-both",
			args{
				v12.ResourceRequirements{
					Limits: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
					},
					Requests: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
					},
				},
			},
			map[v12.ResourceName][]v12.Toleration{
				v12.ResourceStorage: {tolStorage},
				ResourceNvidiaGPU:   {tolGPU},
			},
			[]v12.Toleration{tolStorage},
		},
		{
			"no-tolerations-both",
			args{
				v12.ResourceRequirements{
					Limits: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
						ResourceNvidiaGPU:   resource.MustParse("1"),
					},
					Requests: v12.ResourceList{
						v12.ResourceCPU:     resource.MustParse("1024m"),
						v12.ResourceStorage: resource.MustParse("100M"),
					},
				},
			},
			map[v12.ResourceName][]v12.Toleration{
				v12.ResourceStorage: {tolStorage},
				ResourceNvidiaGPU:   {tolGPU},
			},
			[]v12.Toleration{tolStorage, tolGPU},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, config.SetK8sPluginConfig(&config.K8sPluginConfig{ResourceTolerations: tt.setVal}))
			if got := GetTolerationsForResources(tt.args.resources); len(got) != len(tt.want) {
				t.Errorf("GetTolerationsForResources() = %v, want %v", got, tt.want)
			} else {
				for _, tol := range tt.want {
					assert.Contains(t, got, tol)
				}
			}
		})
	}
}

var testTaskExecutionIdentifier = core.TaskExecutionIdentifier{
	TaskId: &core.Identifier{
		ResourceType: core.ResourceType_TASK,
		Project:      "proj",
		Domain:       "domain",
		Name:         "name",
	},
	NodeExecutionId: &core.NodeExecutionIdentifier{
		NodeId: "nodeId",
		ExecutionId: &core.WorkflowExecutionIdentifier{
			Project: "proj",
			Domain:  "domain",
			Name:    "name",
		},
	},
}

type mockTaskExecutionIdentifier struct{}

func (m mockTaskExecutionIdentifier) GetID() core.TaskExecutionIdentifier {
	return testTaskExecutionIdentifier
}

func (m mockTaskExecutionIdentifier) GetGeneratedName() string {
	return "task-exec-name"
}

func TestDecorateEnvVars(t *testing.T) {
	ctx := context.Background()
	ctx = contextutils.WithWorkflowID(ctx, "fake_workflow")

	defaultEnv := []v12.EnvVar{
		{
			Name:  "x",
			Value: "y",
		},
	}
	additionalEnv := map[string]string{
		"k": "v",
	}
	var emptyEnvVar map[string]string

	expected := append(defaultEnv, GetContextEnvVars(ctx)...)
	expected = append(expected, GetExecutionEnvVars(mockTaskExecutionIdentifier{})...)

	aggregated := append(expected, v12.EnvVar{Name: "k", Value: "v"})
	type args struct {
		envVars []v12.EnvVar
		id      pluginsCore.TaskExecutionID
	}
	tests := []struct {
		name           string
		args           args
		additionEnvVar map[string]string
		want           []v12.EnvVar
	}{
		{"no-additional", args{envVars: defaultEnv, id: mockTaskExecutionIdentifier{}}, emptyEnvVar, expected},
		{"with-additional", args{envVars: defaultEnv, id: mockTaskExecutionIdentifier{}}, additionalEnv, aggregated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, config.SetK8sPluginConfig(&config.K8sPluginConfig{DefaultEnvVars: tt.additionEnvVar}))
			if got := DecorateEnvVars(ctx, tt.args.envVars, tt.args.id); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DecorateEnvVars() = %v, want %v", got, tt.want)
			}
		})
	}
}