package nodepoolmanager

import (
	"context"
	"errors"
)

var (
	ErrDriverNotRegistered = errors.New("driver not found")

	drivers = make(map[string]Driver)
)

type Provider interface {
	ResizeNode(ctx context.Context, count int) error
	DeleteNodes(ctx context.Context, IDs []string) error
}

type Driver interface {
	Connect(config interface{}) (Provider, error)
}

func New(driver string, config interface{}) (Provider, error) {
	if drv, ok := drivers[driver]; ok {
		p, err := drv.Connect(config)
		if err != nil {
			return nil, err
		}
		return p, nil
	}
	return nil, ErrDriverNotRegistered
}

func RegisterDriver(name string, driver Driver) {
	drivers[name] = driver
}
