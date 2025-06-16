package runner

import (
	"context"
	"log"
	"time"

	"github.com/Azure/cluster-health-monitor/pkg/checker"
	"github.com/Azure/cluster-health-monitor/pkg/config"
	"golang.org/x/sync/errgroup"
)

// Runner manages and runs a set of checkers periodically.
type Runner struct {
	config     *config.Config
	chkBuilder CheckerBuilder
}

// CheckerBuilder is a factory function type that builds a Checker from a configuration.
type CheckerBuilder func(cfg config.CheckerConfig) (checker.Checker, error)

func NewRunner(cfg *config.Config) (*Runner, error) {
	return &Runner{
		chkBuilder: nil, // TODO: set a default checker builder after refactoring checker's package
		config:     cfg,
	}, nil
}

// Run starts all checkers according to their configured intervals and timeouts.
// Run create a new checker for each interval instead of reusing the same instance.
func (r *Runner) Run(ctx context.Context) error {
	var g errgroup.Group
	for _, chkCfg := range r.config.Checkers {
		cfg := chkCfg // capture range variable
		g.Go(func() error {
			return r.scheduleChecker(ctx, cfg)
		})
	}
	return g.Wait()
}

func (r *Runner) scheduleChecker(ctx context.Context, cfg config.CheckerConfig) error {
	interval := cfg.Interval
	timeout := cfg.Timeout

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			func() {
				runCtx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()
				chk, err := r.chkBuilder(cfg)
				if err != nil {
					log.Printf("Failed to build checker %q: %v", cfg.Name, err)
					return
				}
				if err := chk.Run(runCtx); err != nil {
					// TODO: handle the error of the checker and emit corresponding metrics
					log.Printf("Checker %q failed: %s", chk.Name(), err)
				}
				// TODO: handle the result of the checker and emit corresponding metrics
			}()

		case <-ctx.Done():
			log.Println("stopping")
			return ctx.Err()
		}
	}
}
