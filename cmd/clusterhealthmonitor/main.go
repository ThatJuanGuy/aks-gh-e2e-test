package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/Azure/cluster-health-monitor/pkg/checker"
	"github.com/Azure/cluster-health-monitor/pkg/checker/dnscheck"
	"github.com/Azure/cluster-health-monitor/pkg/checker/podstartup"
	"github.com/Azure/cluster-health-monitor/pkg/config"
	"github.com/Azure/cluster-health-monitor/pkg/metrics"
	"github.com/Azure/cluster-health-monitor/pkg/scheduler"
	"k8s.io/klog/v2"
)

const (
	defaultConfigPath = "/etc/cluster-health-monitor/config.yaml"
)

func init() {
	klog.InitFlags(nil)
}

func main() {
	configPath := flag.String("config", defaultConfigPath, "Path to the configuration file")
	flag.Parse()
	defer klog.Flush()

	klog.InfoS("Started Cluster Health Monitor")
	registerCheckers()

	// Wait for interrupt signal to gracefully shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run the prometheus metrics server.
	m, err := metrics.NewServer(9800)
	if err != nil {
		klog.ErrorS(err, "Failed to create metrics server")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	go func() {
		if err := m.Run(ctx); err != nil {
			klog.ErrorS(err, "Metrics server error")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
	}()

	// Parse the configuration file.
	cfg, err := config.ParseFromFile(*configPath)
	if err != nil {
		klog.ErrorS(err, "Failed to parse config")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	klog.InfoS("Parsed configuration file",
		"path", *configPath,
		"numCheckers", len(cfg.Checkers))

	// Build the checker schedule from the configuration.
	cs, err := buildCheckerSchedule(cfg)
	if err != nil {
		klog.ErrorS(err, "Failed to build checker schedule")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	klog.InfoS("Built checker schedule", "numSchedules", len(cs))

	// Run the scheduler.
	sched := scheduler.NewScheduler(cs)
	go func() {
		if err := sched.Start(ctx); err != nil {
			klog.ErrorS(err, "Scheduler error")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
	}()
	klog.InfoS("Scheduler started")

	<-ctx.Done()
}

func buildCheckerSchedule(cfg *config.Config) ([]scheduler.CheckerSchedule, error) {
	var schedules []scheduler.CheckerSchedule
	for _, chkCfg := range cfg.Checkers {
		chk, err := checker.Build(&chkCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to build checker %q: %w", chkCfg.Name, err)
		}
		schedules = append(schedules, scheduler.CheckerSchedule{
			Interval: chkCfg.Interval,
			Timeout:  chkCfg.Timeout,
			Checker:  chk,
		})
	}
	return schedules, nil
}

func registerCheckers() {
	dnscheck.Register()
	podstartup.Register()
}
