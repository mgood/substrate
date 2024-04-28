package commands

import (
	"context"
	"errors"
)

type DynamicSource struct {
	Sources []Source
}

var _ Source = (*DynamicSource)(nil)

func (c *DynamicSource) Reflect(ctx context.Context) (DefIndex, error) {
	ci := DefIndex{}
	for _, src := range c.Sources {
		dci, err := src.Reflect(ctx)
		if err != nil {
			return nil, err
		}
		for k, v := range dci {
			ci[k] = v
		}
	}

	return ci, nil
}

func (c *DynamicSource) Run(ctx context.Context, name string, p Fields) (Fields, error) {
	for _, src := range c.Sources {
		result, err := src.Run(ctx, name, p)
		if err == nil || !errors.Is(err, ErrNoSuchCommand) {
			return result, err
		}
	}

	return nil, ErrNoSuchCommand
}
