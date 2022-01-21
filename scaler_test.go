package kubescaler

import (
	"context"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/theredrad/kubescaler/mocks"
	"github.com/theredrad/kubescaler/nodepoolmanager"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sT "k8s.io/client-go/testing"
	"strings"
	"testing"
	"time"
)

const (
	nodeSelector  = "role=scalable"
	podLabelName  = "session"
	podLabelValue = "dedicated-server"
	fmtNodeName   = "test-node-%s"
)

var (
	fmtSpecNodeName = fmt.Sprintf("spec.nodeName=%s", fmtNodeName)
)

type requestPod struct {
	cpuResource       string
	ramResource       string
	isDedicatedServer bool
}

func TestScaler_scale(t *testing.T) {
	clientSet := &fake.Clientset{}
	k8s := NewK8S(clientSet)
	nodes := &v1.NodeList{}
	npm := newNodePoolManagerMock(t, clientSet, nodes, false)
	srv := NewScaler(npm, k8s, &Config{
		NodeSelector:   nodeSelector,
		MinimumNode:    2,
		MaximumNode:    6,
		PodCPURequest:  100,
		BufferSlotSize: 4,
		PodLabelName:   podLabelName,
		PodLabelValue:  podLabelValue,
	})

	clientSet.AddReactor("list", "nodes", func(a k8sT.Action) (bool, runtime.Object, error) {
		return true, nodes, nil
	})

	clientSet.AddReactor("update", "nodes", func(a k8sT.Action) (bool, runtime.Object, error) {
		updateAction := a.(k8sT.UpdateAction)
		n := updateAction.GetObject().(*v1.Node)

		for i, node := range nodes.Items {
			if node.Name == n.Name {
				nodes.Items[i] = *n
			}
		}

		return true, n, nil
	})

	tests := []struct {
		name          string
		nodes         []v1.Node
		expectedNodes int
		podsMap       map[string]*v1.PodList
		unscheduled   map[string]bool
	}{
		{
			name:          "minimum_scale",
			nodes:         []v1.Node{},
			expectedNodes: 2,
		},
		{
			name:          "scale_up",
			nodes:         newNodeList(2, false).Items,
			expectedNodes: 3,
			podsMap: map[string]*v1.PodList{
				fmt.Sprintf(fmtSpecNodeName, "0"): newPodList(repeatRequestPod(9, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
				fmt.Sprintf(fmtSpecNodeName, "1"): newPodList(repeatRequestPod(8, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
			},
		},
		{
			name:          "scale_down",
			nodes:         newNodeList(3, false).Items,
			expectedNodes: 2,
			podsMap: map[string]*v1.PodList{
				fmt.Sprintf(fmtSpecNodeName, "0"): newPodList(repeatRequestPod(9, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
				fmt.Sprintf(fmtSpecNodeName, "1"): newPodList(repeatRequestPod(7, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
			},
		},
		{
			name:          "scale_down_unschedule",
			nodes:         newNodeList(3, false).Items,
			expectedNodes: 3,
			podsMap: map[string]*v1.PodList{
				fmt.Sprintf(fmtSpecNodeName, "0"): newPodList(repeatRequestPod(8, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
				fmt.Sprintf(fmtSpecNodeName, "1"): newPodList(repeatRequestPod(7, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
				fmt.Sprintf(fmtSpecNodeName, "2"): newPodList(repeatRequestPod(1, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
			},
			unscheduled: map[string]bool{fmt.Sprintf(fmtNodeName, "2"): true},
		},
		{
			name: "scale_up_schedule",
			nodes: func() []v1.Node {
				tmp := newNodeList(3, false).Items
				n := Node{
					N: &tmp[2],
				}
				err := n.MarkAsUnschedulable()
				if err != nil {
					t.Logf("expected marking node as unschedulable, got error: %s", err)
					t.FailNow()
				}
				return tmp
			}(),
			expectedNodes: 3,
			podsMap: map[string]*v1.PodList{
				fmt.Sprintf(fmtSpecNodeName, "0"): newPodList(repeatRequestPod(9, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
				fmt.Sprintf(fmtSpecNodeName, "1"): newPodList(repeatRequestPod(8, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
				fmt.Sprintf(fmtSpecNodeName, "2"): newPodList(repeatRequestPod(1, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
			},
		},
		{
			name: "scale_down_unschedule_non_dedicated",
			nodes: func() []v1.Node {
				tmp := newNodeList(3, false).Items
				n := Node{
					N: &tmp[2],
				}
				err := n.MarkAsUnschedulable()
				if err != nil {
					t.Logf("expected marking node as unschedulable, got error: %s", err)
					t.FailNow()
				}
				return tmp
			}(),
			expectedNodes: 2,
			podsMap: map[string]*v1.PodList{
				fmt.Sprintf(fmtSpecNodeName, "0"): newPodList(repeatRequestPod(8, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
				fmt.Sprintf(fmtSpecNodeName, "1"): newPodList(repeatRequestPod(7, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: true,
				})),
				fmt.Sprintf(fmtSpecNodeName, "2"): newPodList(repeatRequestPod(1, requestPod{
					cpuResource:       "0.1",
					isDedicatedServer: false, // run 1 non-dedicated server pod, so it should be deleted
				})),
			},
			unscheduled: map[string]bool{fmt.Sprintf(fmtNodeName, "2"): true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes.Items = tt.nodes

			if tt.podsMap != nil {
				clientSet.AddReactor("list", "pods", func(a k8sT.Action) (bool, runtime.Object, error) {
					if list, ok := tt.podsMap[a.(k8sT.ListAction).GetListRestrictions().Fields.String()]; ok {
						return true, list, nil
					}
					return true, &v1.PodList{}, nil
				})
			}

			err := srv.scale()
			if err != nil {
				t.Logf("expected scaler, got err: %s", err)
				t.FailNow()
			}

			tmpNodes, err := srv.k8s.Nodes(context.Background(), srv.config.NodeSelector)
			if err != nil {
				t.Logf("expected node list, got err: %s", err)
				t.FailNow()
			}

			if len(tmpNodes.Nodes) != tt.expectedNodes {
				t.Logf("expected %d nodes, got %d", tt.expectedNodes, len(tmpNodes.Nodes))
				t.FailNow()
			}

			if tt.unscheduled == nil {
				tt.unscheduled = make(map[string]bool)
			}

			for _, n := range tmpNodes.Nodes {
				if _, ok := tt.unscheduled[n.N.Name]; ok == n.IsSchedulable() {
					t.Logf("expected node as unschedulable, got schedulable")
					t.FailNow()
				}
			}
		})
	}
}

func waitForNode(nodes *v1.NodeList, count int) error {
	t := time.NewTicker(500 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		select {
		case <-t.C:
			if len(nodes.Items) == count {
				t.Stop()
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func newNodeList(count int, randomNaming bool) *v1.NodeList {
	list := &v1.NodeList{}

	var i int
	for ; i < count; i++ {
		name := fmt.Sprintf("%d", i)
		if randomNaming {
			name = uuid.New().String()
		}
		list.Items = append(list.Items, v1.Node{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1",
			},
			Spec: v1.NodeSpec{
				Unschedulable: false,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-node-%s", name),
				Namespace: "",
				Labels: map[string]string{
					"role": "scalable",
				},
				Annotations: map[string]string{},
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionTrue,
					},
				},
				Capacity: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.0"),
				},
			},
		})
	}

	return list
}

func newPodList(pods []requestPod) *v1.PodList {
	list := &v1.PodList{}
	for _, p := range pods {
		labels := make(map[string]string)
		if p.isDedicatedServer {
			labels[podLabelName] = podLabelValue
		}
		list.Items = append(list.Items, v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-pod-%s", uuid.New()),
				Namespace: "",
				Labels:    labels,
			},
			Spec: v1.PodSpec{
				NodeSelector: map[string]string{
					"role": "scalable",
				},
				Containers: []v1.Container{
					{
						Name:            "dedicated-server",
						Image:           "dedicated-server",
						ImagePullPolicy: "Always",
						Resources: v1.ResourceRequirements{
							Limits: map[v1.ResourceName]resource.Quantity{
								v1.ResourceCPU: resource.MustParse(p.cpuResource),
							},
						},
					},
				},
			},
		})
	}

	return list
}

func newNodePoolManagerMock(t *testing.T, k8s *fake.Clientset, nodes *v1.NodeList, randomNaming bool) nodepoolmanager.Provider {
	ctrl := gomock.NewController(t)
	npm := mocks.NewMockNodePoolProvider(ctrl)

	npm.EXPECT().ResizeNode(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, count int) error {
		if len(nodes.Items) < count {
			t.Logf("resizing node pool to %d", count)
			newNodes := newNodeList(count-len(nodes.Items), randomNaming)
			nodes.Items = append(nodes.Items, newNodes.Items...)

			k8s.AddReactor("list", "nodes", func(a k8sT.Action) (bool, runtime.Object, error) {
				return true, nodes, nil
			})
		}
		return nil
	}).AnyTimes()

	npm.EXPECT().DeleteNodes(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, IDs []string) error {
		if len(IDs) > 0 {
			t.Logf("deleting nodes: %s", strings.Join(IDs, ","))
			i := make(map[string]bool)
			for _, ID := range IDs {
				i[ID] = true
			}

			var newNodes []v1.Node
			for _, n := range nodes.Items {
				if _, ok := i[n.Name]; !ok {
					newNodes = append(newNodes, n)
				}
			}
			nodes.Items = newNodes
		}
		return nil
	}).AnyTimes()

	return npm
}

func repeatRequestPod(count int, p requestPod) []requestPod {
	var pods []requestPod
	for i := 0; i < count; i++ {
		pods = append(pods, p)
	}
	return pods
}
