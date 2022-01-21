package digitalocean

import (
	"context"
	"errors"
	"github.com/digitalocean/godo"
)

var (
	ErrClusterNotFound  = errors.New("digitalocean provider: cluster not found")
	ErrNodePoolNotFound = errors.New("digitalocean provider: node pool not found")
)

type Provider struct {
	client *godo.Client

	clusterID  string
	nodePoolID string
}

func NewProvider(client *godo.Client, clusterName string, nodePoolName string) (*Provider, error) {
	p := Provider{
		client: client,
	}

	c, err := p.findCluster(context.Background(), clusterName)
	if err != nil {
		return nil, err
	}
	p.clusterID = c.ID

	np, err := p.findNodePool(context.Background(), nodePoolName)
	if err != nil {
		return nil, err
	}
	p.nodePoolID = np.ID

	return &p, nil
}

func (p *Provider) findCluster(ctx context.Context, name string) (*godo.KubernetesCluster, error) {
	// TODO: search in other pages
	clusters, _, err := p.client.Kubernetes.List(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters {
		if cluster.Name == name {
			return cluster, nil
		}
	}

	return nil, ErrClusterNotFound
}

func (p *Provider) findNodePool(ctx context.Context, name string) (*godo.KubernetesNodePool, error) {
	nodePools, _, err := p.client.Kubernetes.ListNodePools(ctx, p.clusterID, &godo.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, nodePool := range nodePools {
		if nodePool.Name == name {
			return nodePool, nil
		}
	}

	return nil, ErrNodePoolNotFound
}

func (p *Provider) ResizeNode(ctx context.Context, count int) error {
	_, _, err := p.client.Kubernetes.UpdateNodePool(ctx, p.clusterID, p.nodePoolID, &godo.KubernetesNodePoolUpdateRequest{
		Count: &count,
	})
	return err
}

func (p *Provider) DeleteNodes(ctx context.Context, IDs []string) error {
	for _, ID := range IDs {
		_, err := p.client.Kubernetes.DeleteNode(ctx, p.clusterID, p.nodePoolID, ID, &godo.KubernetesNodeDeleteRequest{})
		return err
	}
	return nil
}
