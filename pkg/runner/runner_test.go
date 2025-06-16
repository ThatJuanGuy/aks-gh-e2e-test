package runner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/cluster-health-monitor/pkg/checker"
	"github.com/Azure/cluster-health-monitor/pkg/config"
	. "github.com/onsi/gomega"
)

type fakeChecker struct {
	name     string
	runCount *int32
	runErr   error
	delay    time.Duration
}

func (f *fakeChecker) Name() string { return f.name }
func (f *fakeChecker) Run(ctx context.Context) error {
	atomic.AddInt32(f.runCount, 1)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.runErr
}

func fakeBuilderFactory(runCount *int32, runErr error, delay time.Duration) CheckerBuilder {
	return func(cfg config.CheckerConfig) (checker.Checker, error) {
		return &fakeChecker{name: cfg.Name, runCount: runCount, runErr: runErr, delay: delay}, nil
	}
}

func TestRunner_Run_Periodic(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	var runCount int32
	cfg := &config.Config{
		Checkers: []config.CheckerConfig{
			{
				Name:     "periodic1",
				Type:     "fake",
				Interval: 50 * time.Millisecond,
				Timeout:  0,
			},
		},
	}
	runner := &Runner{
		config:     cfg,
		chkBuilder: fakeBuilderFactory(&runCount, nil, 0),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 160*time.Millisecond)
	defer cancel()
	_ = runner.Run(ctx)
	g.Expect(runCount).To(BeNumerically(">=", 2))
}
