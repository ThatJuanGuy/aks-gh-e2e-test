package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/Azure/cluster-health-monitor/pkg/checker/dnscheck"
	"github.com/Azure/cluster-health-monitor/pkg/checker/podstartup"
	"github.com/Azure/cluster-health-monitor/pkg/metrics"
	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(nil)
}

func main() {
	flag.Parse()
	defer klog.Flush()

	registerCheckers()

	ctx := context.Background()

	// Wait for interrupt signal to gracefully shutdown
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run the prometheus metrics server
	m, err := metrics.NewServer(9800)
	if err != nil {
		log.Fatalf("Failed to create metrics server:%s.", err)
	}
	go func() {
		if err := m.Run(ctx); err != nil {
			log.Fatalf("Metrics server error: %v.", err)
		}
	}()

	// TODO: run the Scheduler
	<-ctx.Done()
}

func registerCheckers() {
	dnscheck.Register()
	podstartup.Register()
}
