// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package collectors

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/tagger/utils"
	"github.com/DataDog/datadog-agent/pkg/util/kubernetes"
	"github.com/DataDog/datadog-agent/pkg/util/kubernetes/clustername"
	"github.com/DataDog/datadog-agent/pkg/workloadmeta"
	workloadmetatesting "github.com/DataDog/datadog-agent/pkg/workloadmeta/testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleKubePod(t *testing.T) {
	const (
		fullyFleshedContainerID = "foobarquux"
		containerName           = "agent"
		runtimeContainerName    = "k8s_datadog-agent_agent"
		podName                 = "datadog-agent-foobar"
		podNamespace            = "default"
		env                     = "production"
		svc                     = "datadog-agent"
		version                 = "7.32.0"
	)

	standardTags := []string{
		fmt.Sprintf("env:%s", env),
		fmt.Sprintf("service:%s", svc),
		fmt.Sprintf("version:%s", version),
	}

	podEntityID := workloadmeta.EntityID{
		Kind: workloadmeta.KindKubernetesPod,
		ID:   "foobar",
	}

	podTaggerEntityID := fmt.Sprintf("kubernetes_pod_uid://%s", podEntityID.ID)

	tests := []struct {
		name              string
		labelsAsTags      map[string]string
		annotationsAsTags map[string]string
		nsLabelsAsTags    map[string]string
		pod               workloadmeta.KubernetesPod
		expected          *TagInfo
	}{
		{
			name: "fully formed pod",
			annotationsAsTags: map[string]string{
				"gitcommit": "+gitcommit",
				"component": "component",
			},
			labelsAsTags: map[string]string{
				"ownerteam": "team",
				"tier":      "tier",
			},
			nsLabelsAsTags: map[string]string{
				"ns_env":       "ns_env",
				"ns-ownerteam": "ns-team",
			},
			pod: workloadmeta.KubernetesPod{
				EntityID: podEntityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name:      podName,
					Namespace: podNamespace,
					Annotations: map[string]string{
						// Annotations as tags
						"GitCommit": "foobar",
						"ignoreme":  "ignore",
						"component": "agent",

						// Custom tags from map
						"ad.datadoghq.com/tags": `{"pod_template_version":"1.0.0"}`,
					},
					Labels: map[string]string{
						// Labels as tags
						"OwnerTeam":         "container-integrations",
						"tier":              "node",
						"pod-template-hash": "490794276",

						// Standard tags
						"tags.datadoghq.com/env":     env,
						"tags.datadoghq.com/service": svc,
						"tags.datadoghq.com/version": version,

						// K8s recommended tags
						"app.kubernetes.io/name":       svc,
						"app.kubernetes.io/instance":   podName,
						"app.kubernetes.io/version":    version,
						"app.kubernetes.io/component":  "agent",
						"app.kubernetes.io/part-of":    "datadog",
						"app.kubernetes.io/managed-by": "helm",
					},
				},

				// NS labels as tags
				NamespaceLabels: map[string]string{
					"ns_env":       "dev",
					"ns-ownerteam": "containers",
					"foo":          "bar",
				},

				// kube_service tags
				KubeServices: []string{"service1", "service2"},

				// Owner tags
				Owners: []workloadmeta.KubernetesPodOwner{
					{
						Kind: kubernetes.DeploymentKind,
						Name: svc,
					},
				},

				// PVC tags
				PersistentVolumeClaimNames: []string{"pvc-0"},

				// Phase tags
				Phase: "Running",
			},
			expected: &TagInfo{
				Entity: podTaggerEntityID,
				HighCardTags: []string{
					"gitcommit:foobar",
				},
				OrchestratorCardTags: []string{
					fmt.Sprintf("pod_name:%s", podName),
					"kube_ownerref_name:datadog-agent",
				},
				LowCardTags: append([]string{
					fmt.Sprintf("kube_app_instance:%s", podName),
					fmt.Sprintf("kube_app_name:%s", svc),
					fmt.Sprintf("kube_app_version:%s", version),
					fmt.Sprintf("kube_deployment:%s", svc),
					fmt.Sprintf("kube_namespace:%s", podNamespace),
					"component:agent",
					"kube_app_component:agent",
					"kube_app_managed_by:helm",
					"kube_app_part_of:datadog",
					"kube_ownerref_kind:deployment",
					"kube_service:service1",
					"kube_service:service2",
					"ns-team:containers",
					"ns_env:dev",
					"pod_phase:running",
					"pod_template_version:1.0.0",
					"team:container-integrations",
					"tier:node",
				}, standardTags...),
				StandardTags: standardTags,
			},
		},
		{
			name: "pod from openshift deployment",
			pod: workloadmeta.KubernetesPod{
				EntityID: podEntityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name:      podName,
					Namespace: podNamespace,
					Annotations: map[string]string{
						"openshift.io/deployment-config.latest-version": "1",
						"openshift.io/deployment-config.name":           "gitlab-ce",
						"openshift.io/deployment.name":                  "gitlab-ce-1",
					},
				},
			},
			expected: &TagInfo{
				Entity:       podTaggerEntityID,
				HighCardTags: []string{},
				OrchestratorCardTags: []string{
					fmt.Sprintf("pod_name:%s", podName),
					"oshift_deployment:gitlab-ce-1",
				},
				LowCardTags: append([]string{
					fmt.Sprintf("kube_namespace:%s", podNamespace),
					"oshift_deployment_config:gitlab-ce",
				}),
				StandardTags: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &WorkloadMetaCollector{}

			collector.initPodMetaAsTags(tt.labelsAsTags, tt.annotationsAsTags, tt.nsLabelsAsTags)

			actual := collector.handleKubePod(workloadmeta.Event{
				Type:   workloadmeta.EventTypeSet,
				Entity: &tt.pod,
			})

			assertTagInfoEqual(t, tt.expected, actual)
		})
	}
}

func TestHandleECSTask(t *testing.T) {
	entityID := workloadmeta.EntityID{
		Kind: workloadmeta.KindECSTask,
		ID:   "foobar",
	}

	tests := []struct {
		name     string
		task     workloadmeta.ECSTask
		expected *TagInfo
	}{
		{
			name: "basic ECS Fargate task",
			task: workloadmeta.ECSTask{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: "foobar",
				},
				LaunchType: workloadmeta.ECSLaunchTypeFargate,
			},
			expected: &TagInfo{
				Entity:       OrchestratorScopeEntityID,
				HighCardTags: []string{},
				OrchestratorCardTags: []string{
					"task_arn:foobar",
				},
				LowCardTags:  []string{},
				StandardTags: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &WorkloadMetaCollector{
				collectEC2ResourceTags: true,
			}

			actual := collector.handleECSTask(workloadmeta.Event{
				Type:   workloadmeta.EventTypeSet,
				Entity: &tt.task,
			})

			assertTagInfoEqual(t, tt.expected, actual)
		})
	}
}

func TestHandleContainer(t *testing.T) {
	const (
		containerID   = "foobar"
		containerName = "k8s-foobar-datadog-agent"

		podID            = "foobar"
		podName          = "datadog-agent-deadbeef"
		podNamespace     = "default"
		podContainerName = "agent"

		taskID            = "foobar"
		taskContainerName = "agent"

		env     = "production"
		svc     = "datadog-agent"
		version = "7.32.0"
	)

	podImage := workloadmeta.ContainerImage{
		ID:        "datadog/agent@sha256:a63d3f66fb2f69d955d4f2ca0b229385537a77872ffc04290acae65aed5317d2",
		RawName:   "datadog/agent@sha256:a63d3f66fb2f69d955d4f2ca0b229385537a77872ffc04290acae65aed5317d2",
		Name:      "datadog/agent",
		ShortName: "agent",
		Tag:       "latest",
	}

	image := workloadmeta.ContainerImage{
		ID:        "docker.io/datadog/agent@sha256:a63d3f66fb2f69d955d4f2ca0b229385537a77872ffc04290acae65aed5317d2",
		RawName:   "docker.io/datadog/agent@sha256:a63d3f66fb2f69d955d4f2ca0b229385537a77872ffc04290acae65aed5317d2",
		Name:      "docker.io/datadog/agent",
		ShortName: "agent",
		Tag:       "latest",
	}

	store := workloadmetatesting.NewStore()
	store.Set(&workloadmeta.KubernetesPod{
		EntityID: workloadmeta.EntityID{
			Kind: workloadmeta.KindKubernetesPod,
			ID:   podID,
		},
		EntityMeta: workloadmeta.EntityMeta{
			Name:      podName,
			Namespace: podNamespace,
			Labels: map[string]string{
				"tags.datadoghq.com/agent.env":     env,
				"tags.datadoghq.com/agent.service": svc,
				"tags.datadoghq.com/agent.version": version,
			},
		},
		Containers: map[string]workloadmeta.OrchestratorContainer{
			containerID: {
				ID:    containerID,
				Name:  podContainerName,
				Image: podImage,
			},
		},
	})
	store.Set(&workloadmeta.ECSTask{
		EntityID: workloadmeta.EntityID{
			Kind: workloadmeta.KindECSTask,
			ID:   taskID,
		},
		EntityMeta: workloadmeta.EntityMeta{
			Name: "foobar",
		},
		Tags: map[string]string{
			"aws:ecs:clusterName": "ecs-cluster",
			"aws:ecs:serviceName": "datadog-agent",
			"owner_team":          "container-integrations",
		},
		ContainerInstanceTags: map[string]string{
			"instance_type": "g4dn.xlarge",
		},
		ClusterName: "ecs-cluster",
		Family:      "datadog-agent",
		Version:     "1",
		LaunchType:  workloadmeta.ECSLaunchTypeEC2,
		Containers: map[string]workloadmeta.OrchestratorContainer{
			containerID: {
				ID:   containerID,
				Name: taskContainerName,
			},
		},
	})

	standardTags := []string{
		fmt.Sprintf("env:%s", env),
		fmt.Sprintf("service:%s", svc),
		fmt.Sprintf("version:%s", version),
	}

	entityID := workloadmeta.EntityID{
		Kind: workloadmeta.KindContainer,
		ID:   containerID,
	}

	taggerEntityID := fmt.Sprintf("container_id://%s", entityID.ID)

	tests := []struct {
		name         string
		labelsAsTags map[string]string
		envAsTags    map[string]string
		container    workloadmeta.Container
		expected     *TagInfo
	}{
		{
			name: "fully formed container",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
					Labels: map[string]string{
						"com.datadoghq.tags.env":     env,
						"com.datadoghq.tags.service": svc,
						"com.datadoghq.tags.version": version,
					},
				},
				Runtime: workloadmeta.ContainerRuntimeDocker,
				Image:   image,
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("container_id:%s", entityID.ID),
				},
				OrchestratorCardTags: []string{},
				LowCardTags: append([]string{
					fmt.Sprintf("docker_image:%s:%s", image.Name, image.Tag),
					fmt.Sprintf("image_name:%s", image.Name),
					fmt.Sprintf("image_tag:%s", image.Tag),
					fmt.Sprintf("short_image:%s", image.ShortName),
				}, standardTags...),
				StandardTags: standardTags,
			},
		},
		{
			name: "tags from environment",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
				},
				EnvVars: map[string]string{
					// env as tags
					"TEAM": "container-integrations",
					"TIER": "node",

					// standard tags
					"DD_ENV":     env,
					"DD_SERVICE": svc,
					"DD_VERSION": version,
				},
			},
			envAsTags: map[string]string{
				"team": "owner_team",
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("container_id:%s", entityID.ID),
				},
				OrchestratorCardTags: []string{},
				LowCardTags: append([]string{
					"owner_team:container-integrations",
				}, standardTags...),
				StandardTags: standardTags,
			},
		},
		{
			name: "tags from labels",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
					Labels: map[string]string{
						// labels as tags
						"team": "container-integrations",
						"tier": "node",

						// custom tags from label
						"com.datadoghq.ad.tags": `["app_name:datadog-agent"]`,
					},
				},
			},
			labelsAsTags: map[string]string{
				"team": "owner_team",
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("container_id:%s", entityID.ID),
					"app_name:datadog-agent",
				},
				OrchestratorCardTags: []string{},
				LowCardTags: append([]string{
					"owner_team:container-integrations",
				}),
				StandardTags: []string{},
			},
		},
		{
			name: "tags from labels and envs with prefix (using *)",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
					Labels: map[string]string{
						"team": "container-integrations",
					},
				},
				EnvVars: map[string]string{
					"some_env": "some_env_val",
				},
			},
			labelsAsTags: map[string]string{
				"*": "custom_label_prefix_%%label%%",
			},
			envAsTags: map[string]string{
				"*": "custom_env_prefix_%%env%%",
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("container_id:%s", entityID.ID),
				},
				OrchestratorCardTags: []string{},
				LowCardTags: append([]string{
					// Notice that the names include the custom prefixes
					// added in labelsAsTags and envAsTags.
					"custom_label_prefix_team:container-integrations",
					"custom_env_prefix_some_env:some_env_val",
				}),
				StandardTags: []string{},
			},
		},
		{
			name: "docker container with image that has no tag",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
				},
				Runtime: workloadmeta.ContainerRuntimeDocker,
				Image: workloadmeta.ContainerImage{
					RawName:   "redis",
					Name:      "redis",
					ShortName: "redis",
					Tag:       "",
				},
			},
			expected: []*TagInfo{
				{
					Entity: taggerEntityID,
					HighCardTags: []string{
						fmt.Sprintf("container_name:%s", containerName),
						fmt.Sprintf("container_id:%s", entityID.ID),
					},
					OrchestratorCardTags: []string{},
					LowCardTags: append([]string{
						"docker_image:redis", // Notice that there's no tag
						"image_name:redis",
						"short_image:redis",
						fmt.Sprintf("container_id:%s", entityID.ID),
						fmt.Sprintf("container_name:%s", containerName),
						fmt.Sprintf("display_container_name:%s_%s", podContainerName, podName),
					},
					OrchestratorCardTags: []string{
						fmt.Sprintf("pod_name:%s", podName),
					},
					LowCardTags: append([]string{
						fmt.Sprintf("image_id:%s", podImage.ID),
						fmt.Sprintf("image_name:%s", podImage.Name),
						fmt.Sprintf("image_tag:%s", podImage.Tag),
						fmt.Sprintf("kube_container_name:%s", podContainerName),
						fmt.Sprintf("kube_namespace:%s", podNamespace),
						fmt.Sprintf("short_image:%s", podImage.ShortName),
					}, standardTags...),
					StandardTags: standardTags,
		},
	},
		{
			name: "k8s container",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
					Labels: map[string]string{
						"io.kubernetes.pod.uid": podID,
					},
				},
				Image: image,
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_id:%s", entityID.ID),
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("display_container_name:%s_%s", podContainerName, podName),
				},
				OrchestratorCardTags: []string{
					fmt.Sprintf("pod_name:%s", podName),
				},
				LowCardTags: append([]string{
					fmt.Sprintf("image_id:%s", podImage.ID),
					fmt.Sprintf("image_name:%s", podImage.Name),
					fmt.Sprintf("image_tag:%s", podImage.Tag),
					fmt.Sprintf("kube_container_name:%s", podContainerName),
					fmt.Sprintf("kube_namespace:%s", podNamespace),
					fmt.Sprintf("short_image:%s", podImage.ShortName),
				}, standardTags...),
				StandardTags: standardTags,
			},
		},
		{
			name: "ecs container",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
					Labels: map[string]string{
						"com.amazonaws.ecs.task-arn": taskID,
					},
				},
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_id:%s", entityID.ID),
					fmt.Sprintf("container_name:%s", containerName),
				},
				OrchestratorCardTags: []string{
					"task_arn:foobar",
				},
				LowCardTags: append([]string{
					"cluster_name:ecs-cluster",
					"ecs_cluster_name:ecs-cluster",
					"ecs_container_name:agent",
					"instance_type:g4dn.xlarge",
					"owner_team:container-integrations",
					"task_family:datadog-agent",
					"task_name:datadog-agent",
					"task_version:1",
				}),
				StandardTags: []string{},
			},
		},
		{
			name: "nomad container",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
				},
				EnvVars: map[string]string{
					"NOMAD_TASK_NAME":  "test-task",
					"NOMAD_JOB_NAME":   "test-job",
					"NOMAD_GROUP_NAME": "test-group",
					"NOMAD_NAMESPACE":  "test-namespace",
					"NOMAD_DC":         "test-dc",
				},
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("container_id:%s", entityID.ID),
				},
				OrchestratorCardTags: []string{},
				LowCardTags: append([]string{
					"nomad_task:test-task",
					"nomad_job:test-job",
					"nomad_group:test-group",
					"nomad_namespace:test-namespace",
					"nomad_dc:test-dc",
				}),
				StandardTags: []string{},
			},
		},
		{
			name: "mesos dc/os container",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
				},
				EnvVars: map[string]string{
					"MARATHON_APP_ID":   "/system/dd-agent",
					"CHRONOS_JOB_NAME":  "app1_process-orders",
					"CHRONOS_JOB_OWNER": "qa",
					"MESOS_TASK_ID":     "system_dd-agent.dcc75b42-4b87-11e7-9a62-70b3d5800001",
				},
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("container_id:%s", entityID.ID),
				},
				OrchestratorCardTags: []string{
					"mesos_task:system_dd-agent.dcc75b42-4b87-11e7-9a62-70b3d5800001",
				},
				LowCardTags: append([]string{
					"chronos_job:app1_process-orders",
					"chronos_job_owner:qa",
					"marathon_app:/system/dd-agent",
				}),
				StandardTags: []string{},
			},
		},
		{
			name: "rancher container",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
					Labels: map[string]string{
						"io.rancher.cni.network":             "ipsec",
						"io.rancher.cni.wait":                "true",
						"io.rancher.container.ip":            "10.42.234.7/16",
						"io.rancher.container.mac_address":   "02:f1:dd:48:4c:d9",
						"io.rancher.container.name":          "testAD-redis-1",
						"io.rancher.container.pull_image":    "always",
						"io.rancher.container.uuid":          "8e969193-2bc7-4a58-9a54-9eed44b01bb2",
						"io.rancher.environment.uuid":        "adminProject",
						"io.rancher.project.name":            "testAD",
						"io.rancher.project_service.name":    "testAD/redis",
						"io.rancher.service.deployment.unit": "06c082fc-4b66-4b6c-b098-30dbf29ed204",
						"io.rancher.service.launch.config":   "io.rancher.service.primary.launch.config",
						"io.rancher.stack.name":              "testAD",
						"io.rancher.stack_service.name":      "testAD/redis",
					},
				},
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("container_id:%s", entityID.ID),
					"rancher_container:testAD-redis-1",
				},
				OrchestratorCardTags: []string{},
				LowCardTags: append([]string{
					"rancher_service:testAD/redis",
					"rancher_stack:testAD",
				}),
				StandardTags: []string{},
			},
		},
		{
			name: "docker swarm container",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
					Labels: map[string]string{
						"com.docker.swarm.node.id":      "zdtab51ei97djzrpa1y2tz8li",
						"com.docker.swarm.service.id":   "tef96xrdmlj82c7nt57jdntl8",
						"com.docker.swarm.service.name": "helloworld",
						"com.docker.swarm.task":         "",
						"com.docker.swarm.task.id":      "knk1rz1szius7pvyznn9zolld",
						"com.docker.swarm.task.name":    "helloworld.1.knk1rz1szius7pvyznn9zolld",
						"com.docker.stack.namespace":    "default",
					},
				},
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("container_id:%s", entityID.ID),
				},
				OrchestratorCardTags: []string{},
				LowCardTags: append([]string{
					"swarm_namespace:default",
					"swarm_service:helloworld",
				}),
				StandardTags: []string{},
			},
		},
		{
			name: "opencontainers image revision",
			container: workloadmeta.Container{
				EntityID: entityID,
				EntityMeta: workloadmeta.EntityMeta{
					Name: containerName,
					Labels: map[string]string{
						"org.opencontainers.image.revision": "758691a28aa920070651d360814c559bc26af907",
					},
				},
			},
			expected: &TagInfo{
				Entity: taggerEntityID,
				HighCardTags: []string{
					fmt.Sprintf("container_name:%s", containerName),
					fmt.Sprintf("container_id:%s", entityID.ID),
				},
				OrchestratorCardTags: []string{},
				LowCardTags:          []string{"git.commit.sha:758691a28aa920070651d360814c559bc26af907"},
				StandardTags:         []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &WorkloadMetaCollector{
				store:                  store,
				collectEC2ResourceTags: true,
			}
			collector.initContainerMetaAsTags(tt.labelsAsTags, tt.envAsTags)

			actual := collector.handleContainer(workloadmeta.Event{
				Type:   workloadmeta.EventTypeSet,
				Entity: &tt.container,
			})

			assertTagInfoEqual(t, tt.expected, actual)
		})
	}
}

func TestHandleDelete(t *testing.T) {
	const (
		podName       = "datadog-agent-foobar"
		podNamespace  = "default"
		containerID   = "foobarquux"
		containerName = "agent"
	)

	podEntityID := workloadmeta.EntityID{
		Kind: workloadmeta.KindKubernetesPod,
		ID:   "foobar",
	}
	pod := &workloadmeta.KubernetesPod{
		EntityID: podEntityID,
		EntityMeta: workloadmeta.EntityMeta{
			Name:      podName,
			Namespace: podNamespace,
		},
	}

	podTaggerEntityID := fmt.Sprintf("kubernetes_pod_uid://%s", podEntityID.ID)

	collector := &WorkloadMetaCollector{}

	collector.handleKubePod(workloadmeta.Event{
		Type:   workloadmeta.EventTypeSet,
		Entity: pod,
	})

	expected := &TagInfo{
		Entity:       podTaggerEntityID,
		DeleteEntity: true,
	}

	actual := collector.handleDelete(workloadmeta.Event{
		Type:   workloadmeta.EventTypeUnset,
		Entity: pod,
	})

	assertTagInfoEqual(t, expected, actual)
}

func TestHandleContainerStaticTags(t *testing.T) {
	collector := &WorkloadMetaCollector{
		staticTags: map[string]string{
			"eks_fargate_node": "foobar",
		},
	}

	container := workloadmeta.Container{
		EntityID: workloadmeta.EntityID{
			Kind: workloadmeta.KindContainer,
			ID:   "foo",
		},
	}

	expected := &TagInfo{
		Entity: fmt.Sprintf("container_id://%s", container.ID),
		HighCardTags: []string{
			fmt.Sprintf("container_id:%s", container.ID),
		},
		OrchestratorCardTags: []string{},
		LowCardTags:          []string{"eks_fargate_node:foobar"},
		StandardTags:         []string{},
	}

	actual := collector.handleContainer(workloadmeta.Event{
		Type:   workloadmeta.EventTypeSet,
		Entity: &container,
	})

	assertTagInfoEqual(t, expected, actual)
}

func TestHandlePodStaticTags(t *testing.T) {
	collector := &WorkloadMetaCollector{
		staticTags: map[string]string{
			"eks_fargate_node":  "node",
			"kube_cluster_name": "cluster",
		},
	}

	pod := workloadmeta.KubernetesPod{
		EntityID: workloadmeta.EntityID{
			Kind: workloadmeta.KindKubernetesPod,
			ID:   "uid",
		},
	}

	expected := []*TagInfo{
		{
			Source:               podSource,
			Entity:               "kubernetes_pod_uid://uid",
			HighCardTags:         []string{},
			OrchestratorCardTags: []string{},
			LowCardTags:          []string{"eks_fargate_node:node", "kube_cluster_name:cluster"},
			StandardTags:         []string{},
		},
	}

	actual := collector.handleKubePod(workloadmeta.Event{
		Type:   workloadmeta.EventTypeSet,
		Entity: &pod,
	})

	assertTagInfoListEqual(t, expected, actual)
}

func TestParseJSONValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{
			name:    "empty json",
			value:   ``,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid json",
			value:   `{key}`,
			want:    nil,
			wantErr: true,
		},
		{
			name:  "invalid value",
			value: `{"key1": "val1", "key2": 0}`,
			want: []string{
				"key1:val1",
			},
			wantErr: false,
		},
		{
			name:  "strings and arrays",
			value: `{"key1": "val1", "key2": ["val2"]}`,
			want: []string{
				"key1:val1",
				"key2:val2",
			},
			wantErr: false,
		},
		{
			name:  "arrays only",
			value: `{"key1": ["val1", "val11"], "key2": ["val2", "val22"]}`,
			want: []string{
				"key1:val1",
				"key1:val11",
				"key2:val2",
				"key2:val22",
			},
			wantErr: false,
		},
		{
			name:  "strings only",
			value: `{"key1": "val1", "key2": "val2"}`,
			want: []string{
				"key1:val1",
				"key2:val2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := utils.NewTagList()
			err := parseJSONValue(tt.value, tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJSONValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			low, _, _, _ := tags.Compute()
			assert.ElementsMatch(t, tt.want, low)
		})
	}
}

func Test_mergeMaps(t *testing.T) {
	tests := []struct {
		name   string
		first  map[string]string
		second map[string]string
		want   map[string]string
	}{
		{
			name:   "no conflict",
			first:  map[string]string{"first-k1": "first-v1", "first-k2": "first-v2"},
			second: map[string]string{"second-k1": "second-v1", "second-k2": "second-v2"},
			want: map[string]string{
				"first-k1":  "first-v1",
				"first-k2":  "first-v2",
				"second-k1": "second-v1",
				"second-k2": "second-v2",
			},
		},
		{
			name:   "conflict",
			first:  map[string]string{"first-k1": "first-v1", "first-k2": "first-v2"},
			second: map[string]string{"first-k2": "second-v1", "second-k2": "second-v2"},
			want: map[string]string{
				"first-k1":  "first-v1",
				"first-k2":  "first-v2",
				"second-k2": "second-v2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualValues(t, tt.want, mergeMaps(tt.first, tt.second))
		})
	}
}

func TestFargateStaticTags(t *testing.T) {
	mockConfig := config.Mock()
	tests := []struct {
		name        string
		loadFunc    func()
		cleanupFunc func()
		want        map[string]string
	}{
		{
			name: "dd tags",
			loadFunc: func() {
				mockConfig.Set("eks_fargate", true)
				mockConfig.Set("tags", "dd_tag1:dd_val1 dd_tag2:dd_val2")
			},
			cleanupFunc: func() { mockConfig.Set("tags", "") },
			want:        map[string]string{"dd_tag1": "dd_val1", "dd_tag2": "dd_val2"},
		},
		{
			name: "eks fargate node",
			loadFunc: func() {
				mockConfig.Set("eks_fargate", true)
				mockConfig.Set("kubernetes_kubelet_nodename", "fargate_node_name")
			},
			cleanupFunc: func() {
				mockConfig.Set("eks_fargate", false)
				mockConfig.Set("kubernetes_kubelet_nodename", "")
			},
			want: map[string]string{"eks_fargate_node": "fargate_node_name"},
		},
		{
			name: "dd tags and eks fargate node",
			loadFunc: func() {
				mockConfig.Set("tags", "dd_tag1:dd_val1 dd_tag2:dd_val2")
				mockConfig.Set("eks_fargate", true)
				mockConfig.Set("kubernetes_kubelet_nodename", "fargate_node_name")
			},
			cleanupFunc: func() {
				mockConfig.Set("tags", "")
				mockConfig.Set("eks_fargate", false)
				mockConfig.Set("kubernetes_kubelet_nodename", "")
			},
			want: map[string]string{"dd_tag1": "dd_val1", "dd_tag2": "dd_val2", "eks_fargate_node": "fargate_node_name"},
		},
		{
			name:        "no tags",
			loadFunc:    func() {},
			cleanupFunc: func() {},
			want:        nil,
		},
		{
			name: "kube cluster name",
			loadFunc: func() {
				clustername.ResetClusterName()
				mockConfig.Set("eks_fargate", true)
				mockConfig.Set("cluster_name", "fargate-cluster-name")
			},
			cleanupFunc: func() {
				mockConfig.Set("eks_fargate", false)
				mockConfig.Set("cluster_name", "")
				clustername.ResetClusterName()
			},
			want: map[string]string{"kube_cluster_name": "fargate-cluster-name"},
		},
		{
			name: "dd tags and kube cluster name, nominal case",
			loadFunc: func() {
				clustername.ResetClusterName()
				mockConfig.Set("tags", "dd_tag1:dd_val1 dd_tag2:dd_val2")
				mockConfig.Set("eks_fargate", true)
				mockConfig.Set("cluster_name", "fargate-cluster-name")
			},
			cleanupFunc: func() {
				mockConfig.Set("tags", "")
				mockConfig.Set("eks_fargate", false)
				mockConfig.Set("cluster_name", "")
				clustername.ResetClusterName()
			},
			want: map[string]string{"dd_tag1": "dd_val1", "dd_tag2": "dd_val2", "kube_cluster_name": "fargate-cluster-name"},
		},
		{
			name: "dd tags and kube cluster name, kube_cluster_name defined in dd tags",
			loadFunc: func() {
				clustername.ResetClusterName()
				mockConfig.Set("tags", "kube_cluster_name:cluster_name_dd_tags")
				mockConfig.Set("eks_fargate", true)
				mockConfig.Set("cluster_name", "cluster_name")
			},
			cleanupFunc: func() {
				mockConfig.Set("tags", "")
				mockConfig.Set("eks_fargate", false)
				mockConfig.Set("cluster_name", "")
				clustername.ResetClusterName()
			},
			want: map[string]string{"kube_cluster_name": "cluster_name_dd_tags"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.loadFunc()
			defer tt.cleanupFunc()

			assert.EqualValues(t, tt.want, fargateStaticTags(context.TODO()))
		})
	}
}

func assertTagInfoEqual(t *testing.T, expected *TagInfo, item *TagInfo) bool {
	t.Helper()
	sort.Strings(expected.LowCardTags)
	sort.Strings(item.LowCardTags)

	sort.Strings(expected.OrchestratorCardTags)
	sort.Strings(item.OrchestratorCardTags)

	sort.Strings(expected.HighCardTags)
	sort.Strings(item.HighCardTags)

	sort.Strings(expected.StandardTags)
	sort.Strings(item.StandardTags)

	return assert.Equal(t, expected, item)
}
