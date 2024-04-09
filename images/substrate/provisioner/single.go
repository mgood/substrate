package provisioner

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"

	"github.com/ajbouh/substrate/images/substrate/activityspec"
)

type Spawner interface {
	Spawn(ctx context.Context, req *activityspec.ServiceSpawnRequest) (*activityspec.ServiceSpawnResponse, <-chan Event, error)
	Shutdown(ctx context.Context, name string, reason error) error
	Peek(ctx context.Context, req *activityspec.ServiceSpawnRequest) (*activityspec.ServiceSpawnResolution, error)
}

type CachingSingleServiceProvisioner struct {
	peekMu                      *sync.Mutex
	spawnMu                     *sync.Mutex
	gen                         int
	cached                      *url.URL
	cachedToken                 *string
	cleanup                     func(reason error)
	provisioned                 *activityspec.ServiceSpawnResponse
	provisionedResolutionDigest string

	spawner Spawner
	req     *activityspec.ServiceSpawnRequest

	logf func(fmt string, values ...any)
}

func NewCachingSingleServiceProvisioner(spawner Spawner, req *activityspec.ServiceSpawnRequest) *CachingSingleServiceProvisioner {
	return &CachingSingleServiceProvisioner{
		peekMu:  &sync.Mutex{},
		spawnMu: &sync.Mutex{},
		logf: func(fmt string, values ...any) {
			log.Printf(fmt, values...)
		},
		spawner: spawner,
		req:     req,
	}
}

func (e *CachingSingleServiceProvisioner) set(provisioned *activityspec.ServiceSpawnResponse, v *url.URL, t *string) func(reason error) {
	e.peekMu.Lock()
	defer e.peekMu.Unlock()

	e.gen++
	copy := *v
	e.cached = &copy
	e.cachedToken = t
	e.provisioned = provisioned
	e.provisionedResolutionDigest = provisioned.ServiceSpawnResolution.Digest()
	// Do this AFTER we've loaded the cache.
	e.cleanup = e.makeCleanup()
	e.logf("action=cache:set gen=%d url=%s", e.gen, v)

	return e.cleanup
}

func (e *CachingSingleServiceProvisioner) get() (*url.URL, *string, bool, func(err error)) {
	e.peekMu.Lock()
	defer e.peekMu.Unlock()

	e.logf("action=cache:get gen=%d url=%s", e.gen, e.cached)
	if e.cached != nil {
		copy := *e.cached
		e.cleanup = e.makeCleanup()
		return &copy, e.cachedToken, true, e.cleanup
	}
	return nil, nil, false, e.cleanup
}

func (e *CachingSingleServiceProvisioner) makeCleanup() func(error) {
	cleanupGen := e.gen

	return func(reason error) {
		e.peekMu.Lock()
		defer e.peekMu.Unlock()
		if e.gen == cleanupGen && e.cached != nil {
			e.cached = nil
			e.cachedToken = nil
			e.provisioned = nil
			e.provisionedResolutionDigest = ""

			e.logf("action=cache:clear gen=%d cleanupGen=%d err=%s", e.gen, cleanupGen, reason)
		} else {
			e.logf("action=cache:staleclear gen=%d cleanupGen=%d err=%s", e.gen, cleanupGen, reason)
		}
	}
}

func (e *CachingSingleServiceProvisioner) Refresh(ctx context.Context) (bool, error) {
	e.peekMu.Lock()
	spawner := e.spawner
	was := e.provisionedResolutionDigest
	provisioned := e.provisioned
	req := e.req
	cleanup := e.cleanup
	e.peekMu.Unlock()

	if was == "" {
		return false, nil
	}

	res, err := spawner.Peek(ctx, req)
	if err != nil {
		return false, err
	}

	now := res.Digest()
	log.Printf("Refresh service:%s name:%s now:%s was:%s", provisioned.ServiceSpawnResolution.ServiceName, provisioned.Name, now, was)

	if now != was {
		log.Printf("prv %#v", &provisioned.ServiceSpawnResolution)
		log.Printf("cur %#v", res)
		reason := fmt.Errorf("digest changed; was %s, now %s", was, now)
		cleanup(reason)
		err := spawner.Shutdown(ctx, provisioned.Name, reason)
		return true, err
	}

	return false, nil
}

func (e *CachingSingleServiceProvisioner) Ensure(ctx context.Context) (*url.URL, bool, func(error), error) {
	e.spawnMu.Lock()
	defer e.spawnMu.Unlock()

	if target, _, ok, cleanup := e.get(); ok {
		return target, false, cleanup, nil
	}

	streamCtx, streamCancel := context.WithCancel(context.Background())

	sres, ch, err := e.spawner.Spawn(streamCtx, e.req)
	if err != nil {
		streamCancel()
		return nil, false, nil, err
	}

	var parsedToken *string
	if sres.BearerToken != nil {
		parsedToken = sres.BearerToken
	}

	parsed, err := url.Parse(sres.BackendURL)
	if err != nil {
		streamCancel()
		return nil, false, nil, err
	}

	cleanup := e.set(sres, parsed, parsedToken)

	ready := false

	for event := range ch {
		log.Printf("event serivce:%s name:%s %#v", sres.ServiceSpawnResolution.ServiceName, sres.Name, event)
		if event.Error() != nil {
			streamCancel()
			return nil, false, nil, fmt.Errorf("backend will never be ready; err=%w", event.Error())
		}

		if event.IsPending() {
			continue
		}

		if event.IsReady() {
			ready = true
			break
		}

		if event.IsGone() {
			streamCancel()
			return nil, false, nil, fmt.Errorf("backend will never be ready; event=%s", event.String())
		}
	}

	if !ready {
		streamCancel()
		return nil, false, nil, fmt.Errorf("status stream ended without ready")
	}

	go func() {
		// Stay subscribed and cleanup once it's gone.
		defer cleanup(fmt.Errorf("backend error or gone"))
		defer streamCancel()
		for event := range ch {
			log.Printf("event serivce:%s name:%s %#v", sres.ServiceSpawnResolution.ServiceName, sres.Name, event)
			if event.Error() != nil || event.IsGone() {
				break
			}
		}
	}()

	return parsed, true, cleanup, nil
}
