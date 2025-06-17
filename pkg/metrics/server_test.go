package metrics

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestIncHealthCounterValue(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	s, _ := NewServer(8080)
	s.IncHealth("dns", "dns1")

	// Gather metrics
	metrics := make(chan prometheus.Metric, 10)
	s.resultCounter.Collect(metrics)
	close(metrics)

	fmt.Println("Gathered metrics:", len(metrics))
	g.Expect(len(metrics)).To(BeNumerically("==", 1))
	m := <-metrics
	var pb dto.Metric
	if err := m.Write(&pb); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}
	labels := map[string]string{}
	for _, l := range pb.Label {
		labels[l.GetName()] = l.GetValue()
	}
	g.Expect(labels).To(HaveKeyWithValue("checker_type", "dns"))
	g.Expect(labels).To(HaveKeyWithValue("checker_name", "dns1"))
	g.Expect(labels).To(HaveKeyWithValue("status", "healthy"))
	g.Expect(labels).To(HaveKeyWithValue("error_code", "healthy"))
}

func TestIncUnhealthCounterValue(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	s, _ := NewServer(8080)
	s.IncUnhealth("dns", "dns2", "timeout")

	// Gather metrics
	metrics := make(chan prometheus.Metric, 10)
	s.resultCounter.Collect(metrics)
	close(metrics)

	m := <-metrics
	var pb dto.Metric
	if err := m.Write(&pb); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	labels := map[string]string{}
	for _, l := range pb.Label {
		labels[l.GetName()] = l.GetValue()
	}
	g.Expect(labels).To(HaveKeyWithValue("checker_type", "dns"))
	g.Expect(labels).To(HaveKeyWithValue("checker_name", "dns2"))
	g.Expect(labels).To(HaveKeyWithValue("status", "unhealthy"))
	g.Expect(labels).To(HaveKeyWithValue("error_code", "timeout"))
	g.Expect(pb.Counter.GetValue()).To(BeNumerically("==", 1))
}

func TestIncUnknownCounterValue(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	s, _ := NewServer(8080)
	s.IncUnknown("dns", "dns3")

	// Gather metrics
	metrics := make(chan prometheus.Metric, 10)
	s.resultCounter.Collect(metrics)
	close(metrics)

	m := <-metrics
	var pb dto.Metric
	if err := m.Write(&pb); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	labels := map[string]string{}
	for _, l := range pb.Label {
		labels[l.GetName()] = l.GetValue()
	}
	g.Expect(labels).To(HaveKeyWithValue("checker_type", "dns"))
	g.Expect(labels).To(HaveKeyWithValue("checker_name", "dns3"))
	g.Expect(labels).To(HaveKeyWithValue("status", "unknown"))
	g.Expect(labels).To(HaveKeyWithValue("error_code", "unknown"))
	g.Expect(pb.Counter.GetValue()).To(BeNumerically("==", 1))
}
