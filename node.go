package kubescaler

import (
	v1 "k8s.io/api/core/v1"
	"time"
)

const (
	timestampAnnotation = "kubescaler/timestamp"
)

type NodeList struct {
	Nodes []*Node
}

type Node struct {
	N    *v1.Node
	Pods []v1.Pod
}

type Resource struct {
	Name  v1.ResourceName
	Value int64
}

func (n *NodeList) AvailableSlot(need Resource) int64 {
	ar := n.AvailableResource(need.Name)
	return ar / need.Value
}

func (n *NodeList) AvailableResource(resource v1.ResourceName) int64 {
	var a int64
	for _, node := range n.AvailableNodes() {
		a += node.AvailableResource(resource)
	}
	return a
}

func (n *NodeList) AvailableNodes() []*Node {
	var nodes []*Node
	for _, node := range n.Nodes {
		if node.IsReady() && node.IsSchedulable() {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (n *NodeList) UnschedulableNodes() []*Node {
	var nodes []*Node
	for _, node := range n.Nodes {
		if !node.IsSchedulable() {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (n *NodeList) SchedulableNodes() []*Node {
	var nodes []*Node
	for _, node := range n.Nodes {
		if node.IsSchedulable() {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (n *Node) AvailableResource(resource v1.ResourceName) int64 {
	return n.ResourceCapacity(resource) - n.UsingResources(resource)
}

func (n *Node) MarkAsSchedulable() error {
	n.N.Spec.Unschedulable = false
	t, err := time.Now().UTC().MarshalText()
	if err != nil {
		return err
	}
	n.N.ObjectMeta.Annotations[timestampAnnotation] = string(t)
	return nil
}

func (n *Node) MarkAsUnschedulable() error {
	n.N.Spec.Unschedulable = true
	t, err := time.Now().UTC().MarshalText()
	if err != nil {
		return err
	}

	n.N.ObjectMeta.Annotations[timestampAnnotation] = string(t)
	return nil
}

func (n *Node) SchedulingMarkTimestamp() (time.Time, error) {
	var t time.Time
	err := t.UnmarshalText([]byte(n.N.ObjectMeta.Annotations[timestampAnnotation]))
	if err != nil {
		return t, err
	}
	return t, nil
}

func (n *Node) IsReady() bool {
	for _, c := range n.N.Status.Conditions {
		if c.Type == v1.NodeReady && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func (n *Node) IsSchedulable() bool {
	return !n.N.Spec.Unschedulable
}

func (n *Node) ResourceCapacity(resource v1.ResourceName) int64 {
	q := n.N.Status.Capacity[resource]
	return q.MilliValue()
}

func (n *Node) UsingResources(resource v1.ResourceName) int64 {
	var total int64
	for _, pod := range n.Pods {
		for _, container := range pod.Spec.Containers {
			q := container.Resources.Limits[resource]
			total += q.MilliValue()
		}
	}
	return total
}
