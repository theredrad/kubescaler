package kubescaler

import (
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/apimachinery/pkg/watch"
	"sort"
)

type Kubernetes interface {
	Nodes(ctx context.Context, selector string) (*NodeList, error)
	UpdateNode(ctx context.Context, node *Node) error
	NewPodWatcher(ctx context.Context, namespace, labelSelector string) (*PodWatcher, error)
}

type K8S struct {
	i kubernetes.Interface
}

func ClientSet(masterURL, kubeConfigPath string) (kubernetes.Interface, error) {
	var c *rest.Config
	var err error
	if masterURL == "" && kubeConfigPath == "" {
		c, err = rest.InClusterConfig()
	} else {
		c, err = clientcmd.BuildConfigFromFlags(masterURL, kubeConfigPath)
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(c)
}

func NewK8SFromKubeConfig(masterURL, kubeConfigPath string) (*K8S, error) {
	i, err := ClientSet(masterURL, kubeConfigPath)
	if err != nil {
		return nil, err
	}
	return NewK8S(i), nil
}

func NewK8S(i kubernetes.Interface) *K8S {
	return &K8S{
		i: i,
	}
}

func (k *K8S) Nodes(ctx context.Context, selector string) (*NodeList, error) {
	nodes, err := k.i.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}

	return k.toNodeList(ctx, nodes)
}

func (k *K8S) UpdateNode(ctx context.Context, node *Node) error {
	_, err := k.i.CoreV1().Nodes().Update(ctx, node.N, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (k *K8S) NodePods(ctx context.Context, nodeName string) (*v1.PodList, error) {
	fs, err := fields.ParseSelector("spec.nodeName=" + nodeName)
	if err != nil {
		return nil, err
	}

	pods, err := k.i.CoreV1().Pods(v1.NamespaceAll).List(ctx, metav1.ListOptions{FieldSelector: fs.String()})
	return pods, err
}

func (k *K8S) NewPodWatcher(ctx context.Context, namespace, labelSelector string) (*PodWatcher, error) {
	w, err := k.i.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return &PodWatcher{
		w: w,
		Events: make(chan watch.Event),
	}, nil
}

func (k *K8S) toNodeList(ctx context.Context, nodeList *v1.NodeList) (*NodeList, error) {
	var nodes []*Node
	for i, nd := range nodeList.Items {

		n := Node{
			N: &nodeList.Items[i],
		}
		pods, err := k.NodePods(ctx, nd.Name)
		if err != nil {
			return nil, err
		}
		n.Pods = pods.Items

		nodes = append(nodes, &n)
	}
	return &NodeList{
		Nodes: nodes,
	}, nil
}

func SortNodesByPods(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return len(nodes[i].Pods) < len(nodes[j].Pods)
	})
}

func SortNodesByPodsDesc(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return len(nodes[j].Pods) < len(nodes[i].Pods)
	})
}
