package runner

import (
	"context"
	"sync"

	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"golang.org/x/sync/errgroup"
)

type runner struct {
	group         *errgroup.Group
	ctx           context.Context
	mux           sync.Mutex
	shutdownFuncs []func()
}

func (r *runner) Go(f func() error) {
	r.group.Go(f)
}

func (r *runner) Defer(f func()) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.shutdownFuncs = append(r.shutdownFuncs, f)
}

func (r *runner) StartService(services ...service.Service) error {
	for _, s := range services {
		err := s.Start(r.ctx, r)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewRunner creates a new Runner.
func NewRunner(ctx context.Context) service.Runner {
	group, ctx := errgroup.WithContext(ctx)
	return &runner{
		group: group,
		ctx:   ctx,
	}
}
