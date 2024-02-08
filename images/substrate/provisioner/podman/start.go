package podmanprovisioner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/ajbouh/substrate/images/substrate/activityspec"
	"github.com/ajbouh/substrate/images/substrate/fs"

	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/specgen"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	ulid "github.com/oklog/ulid/v2"
)

type P struct {
	connect func(ctx context.Context) (context.Context, error)

	namespace  string
	generation string

	internalNetwork string
	externalNetwork string

	hostResourceDirsRoot string
	hostResourceDirsPath []string
	containerResourceDir string

	waitForReadyTimeout time.Duration
	waitForReadyTick    time.Duration

	prep func(h *specgen.SpecGenerator)
}

var _ activityspec.ProvisionDriver = (*P)(nil)

func New(connect func(ctx context.Context) (context.Context, error), namespace, internalNetwork, externalNetwork, hostResourceDirsRoot string, hostResourceDirsPath []string, prep func(h *specgen.SpecGenerator)) *P {
	return &P{
		connect:              connect,
		namespace:            namespace,
		internalNetwork:      internalNetwork,
		externalNetwork:      externalNetwork,
		hostResourceDirsRoot: hostResourceDirsRoot,
		hostResourceDirsPath: hostResourceDirsPath,
		containerResourceDir: "/res",
		waitForReadyTimeout:  2 * time.Minute,
		waitForReadyTick:     500 * time.Millisecond,
		generation:           ulid.Make().String(),
		prep:                 prep,
	}
}

const LabelSubstrateGeneration = "substrate.generation"
const LabelSubstrateNamespace = "substrate.namespace"
const LabelSubstrateActivity = "substrate.activity"
const SubstrateNetworkNamePrefix = "substrate_network_"

func (p *P) dumpLogs(ctx context.Context, containerID string) error {
	ctx, err := p.connect(ctx)
	if err != nil {
		return err
	}

	ch := make(chan string)
	go func(ch chan string) {
		for s := range ch {
			os.Stderr.WriteString(s)
		}
	}(ch)

	defer close(ch)

	err = containers.Logs(ctx, containerID, &containers.LogOptions{
		Stdout: boolPtr(true),
		Stderr: boolPtr(true),
		Follow: boolPtr(true),
	}, ch, ch)
	return err
}

func (p *P) findResourceDir(rd activityspec.ResourceDirDef) (string, error) {
	rdMainPath := path.Join(p.hostResourceDirsRoot, rd.SHA256)
	if _, err := os.Stat(rdMainPath); err == nil {
		return rdMainPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return rdMainPath, err
	}

	// Use existing from path, otherwise fallback to main
	for _, rdRoot := range p.hostResourceDirsPath {
		rdPath := path.Join(rdRoot, rd.SHA256)
		if _, err := os.Stat(rdPath); err == nil {
			return rdPath, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return rdPath, err
		}
	}

	return rdMainPath, nil
}

func (p *P) prepareResourceDirsMounts(as *activityspec.ServiceSpawnResolution) ([]specs.Mount, error) {
	mounts := make([]specs.Mount, 0, len(as.ServiceDefSpawn.ResourceDirs))
	for alias, rd := range as.ServiceDefSpawn.ResourceDirs {
		rdPath, err := p.findResourceDir(rd)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, specs.Mount{
			Type:        "bind",
			Source:      rdPath,
			Destination: path.Join(p.containerResourceDir, alias),
			Options:     []string{"ro"},
		})
	}

	return mounts, nil
}

func (p *P) Spawn(ctx context.Context, as *activityspec.ServiceSpawnResolution) (*activityspec.ServiceSpawnResponse, error) {
	ctx, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}

	spec, _ := as.Format()
	labels := map[string]string{
		LabelSubstrateNamespace:  p.namespace,
		LabelSubstrateGeneration: p.generation,
		LabelSubstrateActivity:   spec,
	}

	s := specgen.NewSpecGenerator(as.ServiceDefSpawn.Image, false)
	s.Remove = true
	s.Env = map[string]string{}
	s.Labels = labels
	s.Command = append([]string{}, as.ServiceDefSpawn.Command...)

	if p.prep != nil {
		p.prep(s)
	}
	s.Networks = map[string]nettypes.PerNetworkOptions{
		p.internalNetwork: nettypes.PerNetworkOptions{},
		p.externalNetwork: nettypes.PerNetworkOptions{},
	}

	includeView := func(viewName string, includeSpaceIDInTarget bool, view *substratefs.SpaceView) {
		targetPrefix := "/spaces/" + viewName
		if includeSpaceIDInTarget {
			targetPrefix += "/" + view.Tip.SpaceID.String()
		}

		treeMountOptions := []string{}
		if view.IsReadOnly {
			treeMountOptions = append(treeMountOptions, "ro")
		}

		s.Mounts = append(s.Mounts,
			specs.Mount{
				Type:        "bind",
				Source:      view.TreePath(),
				Destination: targetPrefix + "/tree",
				Options:     treeMountOptions,
			},
			specs.Mount{
				Type:        "bind",
				Source:      view.OwnerFilePath(),
				Destination: targetPrefix + "/owner",
				Options:     []string{"ro"},
			},
			specs.Mount{
				Type:        "bind",
				Source:      view.AliasFilePath(),
				Destination: targetPrefix + "/alias",
				Options:     []string{"ro"},
			},
		)
	}

	resourcedirMounts, err := p.prepareResourceDirsMounts(as)
	if err != nil {
		return nil, err
	}

	for _, m := range as.ServiceDefSpawn.Mounts {
		s.Mounts = append(s.Mounts,
			specs.Mount{
				Type:        "bind",
				Source:      m.Source,
				Destination: m.Destination,
				Options:     []string{"ro"},
			},
		)
	}

	s.Mounts = append(s.Mounts, resourcedirMounts...)

	for k, v := range as.ServiceDefSpawn.Environment {
		s.Env[k] = v
	}

	// Pull PORT out of env, so it can be used for port forwarding.
	// TODO consider using configured portmappings instead of this weird approach.
	portStr := s.Env["PORT"]
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("bad PORT value: %w", err)
		}
		s.PortMappings = append(s.PortMappings, nettypes.PortMapping{
			ContainerPort: uint16(port),
			Protocol:      "tcp",
		})
	}

	// TODO need to check schema before we know how to interpret a given parameter...
	// Maybe write a method for each interpretation? Can return an error if it's impossible...
	for parameterName, parameterValue := range as.Parameters {
		switch {
		case parameterValue.Space != nil:
			includeView(parameterName, false, parameterValue.Space)
		case parameterValue.Spaces != nil:
			for _, v := range *parameterValue.Spaces {
				includeView(parameterName, true, &v)
			}
		}
	}

	cResp, err := containers.CreateWithSpec(ctx, s, nil)
	if err != nil {
		return nil, err
	}

	if err := containers.Start(ctx, cResp.ID, nil); err != nil {
		return nil, err
	}

	// Stream logs to stderr
	go p.dumpLogs(ctx, cResp.ID)

	inspect, err := containers.Inspect(ctx, cResp.ID, nil)
	if err != nil {
		return nil, err
	}

	// backendIP := inspect.NetworkSettings.Networks[networkName].IPAddress
	backendURL := "http://" + inspect.Config.Hostname + ":" + portStr
	// backendPortMap := inspect.NetworkSettings.Ports[natPort][0]
	// backendURL := "http://host.docker.internal:" + backendPortMap.HostPort

	var bearerToken *string
	// bearerToken = r.BearerToken

	return &activityspec.ServiceSpawnResponse{
		Name: cResp.ID,

		BackendURL:  backendURL + as.ServiceDefSpawn.URLPrefix,
		BearerToken: bearerToken,

		ServiceSpawnResolution: *as,
	}, nil
}