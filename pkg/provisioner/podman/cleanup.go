package podmanprovisioner

import (
	"context"
	"errors"
	"log"

	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/bindings/network"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

func boolPtr(b bool) *bool {
	return &b
}

func (p *P) Cleanup(
	ctx context.Context,
) error {
	ctx, err := p.connect(ctx)
	if err != nil {
		return err
	}

	cntrs, err := containers.List(ctx, nil)
	if err != nil {
		return err
	}

	removeContainers := []entities.ListContainer{}
	for _, container := range cntrs {
		containerNamespace := container.Labels[LabelSubstrateNamespace]
		if containerNamespace == "" || containerNamespace != p.namespace {
			continue
		}

		containerGeneration := container.Labels[LabelSubstrateGeneration]
		if containerGeneration == "" || containerGeneration == p.generation {
			continue
		}

		removeContainers = append(removeContainers, container)
	}

	errs := []error{}
	for _, c := range removeContainers {
		switch c.State {
		case "running", "paused", "restarting":
			log.Printf("stopping container %s", c.ID)
			err := containers.Stop(ctx, c.ID, nil)
			if err != nil {
				errs = append(errs, err)
			}
		}

		log.Printf("removing container %s", c.ID)
		removes, err := containers.Remove(ctx, c.ID, &containers.RemoveOptions{
			// If we remove volumes then our caches will go away when the last container using it is stopped...
			// Volumes: true,
			Depend: boolPtr(true),
			Force:  boolPtr(true),
		})
		if err != nil {
			errs = append(errs, err)
		} else {
			for _, remove := range removes {
				if remove.Err != nil {
					errs = append(errs, remove.Err)
				}
			}
		}
	}

	networks, err := network.List(ctx, nil)
	if err != nil {
		return err
	}

	removeNetworks := []string{}
	for _, net := range networks {
		containerNamespace := net.Labels[LabelSubstrateNamespace]
		if containerNamespace == "" || containerNamespace != p.namespace {
			continue
		}

		networkGeneration := net.Labels[LabelSubstrateGeneration]
		if networkGeneration == "" || networkGeneration == p.generation {
			continue
		}

		removeNetworks = append(removeNetworks, net.ID)
	}

	for _, net := range removeNetworks {
		log.Printf("removing network %s", net)
		removes, err := network.Remove(ctx, net, nil)
		if err != nil {
			errs = append(errs, err)
		} else {
			for _, remove := range removes {
				if remove.Err != nil {
					errs = append(errs, remove.Err)
				}
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
