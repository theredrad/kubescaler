package kubescaler

import (
	"context"
	"errors"
	"fmt"
	"github.com/theredrad/kubescaler/nodepoolmanager"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"math"
	"time"
)

var (
	ErrNotEnoughResources = errors.New("not enough resources")
)

type Config struct {
	NodeSelector string

	MinimumNode int
	MaximumNode int

	PodCPURequest int64
	PodLabelName  string
	PodLabelValue string

	EmptyNodeExpiration time.Duration

	BufferSlotSize int64

	ScaleLoopDuration time.Duration

	Logger Logger
}

type Scaler struct {
	config *Config
	npm    nodepoolmanager.Provider
	k8s    Kubernetes
	pw     *PodWatcher

	stop chan bool
}

func NewScaler(npm nodepoolmanager.Provider, k8s Kubernetes, config *Config) *Scaler {
	if config == nil {
		config = &Config{}
	}

	if config.Logger == nil {
		config.Logger = NewDefaultLogger(nil, nil, nil)
	}

	// TODO: validate config
	return &Scaler{
		npm:    npm,
		k8s:    k8s,
		config: config,
		stop:   make(chan bool, 1),
	}
}

func (s *Scaler) Start() error {
	var err error
	s.pw, err = s.k8s.NewPodWatcher(context.Background(), v1.NamespaceAll, fmt.Sprintf("%s=%s", s.config.PodLabelName, s.config.PodLabelValue))
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(s.config.ScaleLoopDuration * time.Second)
		defer ticker.Stop()

		for {
			select {
			case e := <-s.pw.Events:
				if e.Type == watch.Added || e.Type == watch.Deleted {
					if err = s.scale(); err != nil {
						s.config.Logger.Errorf("error while trying to scale: ", err)
					}
				}
			case <-ticker.C:
				if err = s.scale(); err != nil {
					s.config.Logger.Errorf("error while trying to scale: ", err)
				}
			case <-s.stop:
				return
			}
		}
	}()

	return nil
}

func (s *Scaler) Stop() {
	s.pw.Stop()
	s.stop <- true
}

func (s *Scaler) scale() error {
	s.config.Logger.Debugf("scaling")
	nodes, err := s.k8s.Nodes(context.Background(), s.config.NodeSelector)
	if err != nil {
		return err
	}
	s.config.Logger.Debugf("current nodes: %d, available nodes: %d", len(nodes.Nodes), len(nodes.AvailableNodes()))

	if len(nodes.Nodes) < s.config.MinimumNode {
		s.config.Logger.Infof("current nodes are smaller than minimum size, resizing %d to %d", len(nodes.Nodes), s.config.MinimumNode)
		return s.npm.ResizeNode(context.Background(), s.config.MinimumNode)
	}

	availableSlot := nodes.AvailableSlot(Resource{
		Name:  v1.ResourceCPU,
		Value: s.config.PodCPURequest,
	})
	s.config.Logger.Infof("available slot: %d, buffer size: %d", availableSlot, s.config.BufferSlotSize)
	if availableSlot < s.config.BufferSlotSize {
		if err = s.checkForScheduling(nodes, &Resource{
			Name:  v1.ResourceCPU,
			Value: s.config.PodCPURequest * s.config.BufferSlotSize,
		}); err != nil && !errors.Is(err, ErrNotEnoughResources) {
			return err
		}

		nodes, err = s.k8s.Nodes(context.Background(), s.config.NodeSelector)
		if err != nil {
			return err
		}

		availableSlot = nodes.AvailableSlot(Resource{
			Name:  v1.ResourceCPU,
			Value: s.config.PodCPURequest,
		})
		s.config.Logger.Infof("request to increase node pool size, available slot: %d, buffer size: %d", availableSlot, s.config.BufferSlotSize)
		err = s.increaseNodePoolSize(nodes, &Resource{
			Name:  v1.ResourceCPU,
			Value: s.config.PodCPURequest * (s.config.BufferSlotSize - availableSlot),
		})
	} else if availableSlot > s.config.BufferSlotSize {
		if err = s.checkForUnscheduling(nodes, &Resource{
			Name:  v1.ResourceCPU,
			Value: s.config.PodCPURequest * (availableSlot - s.config.BufferSlotSize),
		}); err != nil {
			return err
		}
	}

	return s.deleteExtraNodes()
}

func (s *Scaler) increaseNodePoolSize(nodes *NodeList, needs ...*Resource) error {
	var maxNeededNodes int
	for _, r := range needs {
		neededNode := int(math.Ceil(float64(r.Value) / float64(nodes.Nodes[0].ResourceCapacity(r.Name)))) // TODO: using first cell as sample node
		s.config.Logger.Debugf("need resource %s: %d, %d nodes", r.Name, r.Value, neededNode)
		if neededNode > maxNeededNodes {
			maxNeededNodes = neededNode
		}
	}

	size := maxNeededNodes + len(nodes.AvailableNodes())
	s.config.Logger.Debugf("needed nodes: %d, current nodes: %d, size: %d", maxNeededNodes, len(nodes.AvailableNodes()), size)
	if size > s.config.MaximumNode {
		size = s.config.MaximumNode
	}

	return s.npm.ResizeNode(context.Background(), size)
}

func (s *Scaler) checkForUnscheduling(nodes *NodeList, extra ...*Resource) error {
	s.config.Logger.Debugf("check for unscheduling: %d nodes, extra %d %s", len(nodes.Nodes), extra[0].Value, extra[0].Name)
	var minExtraNodes int64
	for _, r := range extra {
		extraNode := int64(math.Floor(float64(r.Value) / float64(nodes.Nodes[0].ResourceCapacity(r.Name))))
		if minExtraNodes == 0 {
			minExtraNodes = extraNode
			continue
		}

		if extraNode < minExtraNodes {
			minExtraNodes = extraNode
		}
	}

	s.config.Logger.Debugf("%d extra node(s) exist", minExtraNodes)
	if minExtraNodes <= 0 {
		return nil
	}

	SortNodesByPods(nodes.Nodes)

	extraNodes := nodes.Nodes[0:minExtraNodes]
	for _, n := range extraNodes {
		err := s.markNodeAsUnschedulable(n)
		if err != nil {
			return err
		}
		s.config.Logger.Debugf("node %s marked as unschedulable with %d pod count", n.N.Name, len(n.Pods))
	}
	return nil
}

func (s *Scaler) checkForScheduling(n *NodeList, needs ...*Resource) error {
	s.config.Logger.Debugf("check for scheduling: %d nodes, needs %d %s", len(n.Nodes), needs[0].Value, needs[0].Name)
	nodes := n.UnschedulableNodes()
	if len(nodes) == 0 {
		s.config.Logger.Debugf("no unscheduled node found")
		return ErrNotEnoughResources
	}

	resources := make(map[v1.ResourceName]int64)
	for _, need := range needs {
		resources[need.Name] = need.Value
	}

	SortNodesByPodsDesc(nodes)

	enoughResources := make(map[v1.ResourceName]bool)
	for _, node := range nodes {

		err := s.markNodeAsSchedulable(node)
		if err != nil {
			return err
		}
		s.config.Logger.Debugf("node %s marked as schedulable", node.N.Name)

		for r, v := range resources {
			v -= node.AvailableResource(r)
			if v <= 0 {
				enoughResources[r] = true
			}
			s.config.Logger.Debugf("needed resource: %d %s", v, r.String())
			if len(enoughResources) == len(needs) {
				return nil
			}
		}
	}
	return errors.New("not enough resource")
}

func (s *Scaler) markNodeAsSchedulable(n *Node) error {
	err := n.MarkAsSchedulable()
	if err != nil {
		return err
	}

	return s.k8s.UpdateNode(context.Background(), n)
}

func (s *Scaler) markNodeAsUnschedulable(n *Node) error {
	err := n.MarkAsUnschedulable()
	if err != nil {
		return err
	}

	return s.k8s.UpdateNode(context.Background(), n)
}

func (s *Scaler) deleteExtraNodes() error {
	s.config.Logger.Debugf("checking to delete extra nodes")
	nodes, err := s.k8s.Nodes(context.Background(), s.config.NodeSelector)
	if err != nil {
		return err
	}

	l := len(nodes.Nodes)
	if l <= s.config.MinimumNode {
		s.config.Logger.Debugf("already at minimum node pool size")
		return nil
	}

	var deleteNodes []string
	for _, node := range nodes.UnschedulableNodes() {
		t, err := node.SchedulingMarkTimestamp()
		if err != nil {
			return err
		}

		s.config.Logger.Debugf("checking node %s pods to delete, pods: %d, expired: %t", node.N.Name, len(s.filterPods(node.Pods)), time.Now().After(t.Add(s.config.EmptyNodeExpiration)))
		if len(s.filterPods(node.Pods)) == 0 && time.Now().After(t.Add(s.config.EmptyNodeExpiration)) {
			s.config.Logger.Infof("node %s should delete", node.N.Name)
			deleteNodes = append(deleteNodes, node.N.Name)
			l--
			if l <= s.config.MinimumNode {
				break
			}
		}
	}
	return s.npm.DeleteNodes(nil, deleteNodes)
}

func (s *Scaler) filterPods(pods []v1.Pod) []v1.Pod {
	var filteredPods []v1.Pod
	for _, p := range pods {
		if v, ok := p.ObjectMeta.Labels[s.config.PodLabelName]; ok && v == s.config.PodLabelValue {
			filteredPods = append(filteredPods, p)
		}
	}
	return filteredPods
}
