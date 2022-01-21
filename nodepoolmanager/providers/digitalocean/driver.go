package digitalocean

import (
	"errors"
	"github.com/digitalocean/godo"
	"github.com/theredrad/kubescaler/nodepoolmanager"
)

const (
	DriverName = "digitalocean"
)

var (
	ErrInvalidConfig          = errors.New("invalid config")
	ErrClusterNameIsRequired  = errors.New("digitalocean provider: cluster name is required")
	ErrNodePoolNameIsRequired = errors.New("digitalocean provider: node pool name is required")
	ErrTokenIsRequired        = errors.New("digitalocean provider: token is required")
)

type Driver struct{}

type Config struct {
	Token        string
	ClusterName  string
	NodePoolName string
}

func init() {
	nodepoolmanager.RegisterDriver(DriverName, &Driver{})
}

func (p *Driver) Connect(config interface{}) (nodepoolmanager.Provider, error) {
	c, ok := config.(*Config)
	if !ok {
		return nil, ErrInvalidConfig
	}

	if c.ClusterName == "" {
		return nil, ErrClusterNameIsRequired
	}

	if c.NodePoolName == "" {
		return nil, ErrNodePoolNameIsRequired
	}

	if c.Token == "" {
		return nil, ErrTokenIsRequired
	}

	return NewProvider(godo.NewFromToken(c.Token), c.ClusterName, c.NodePoolName)
}
