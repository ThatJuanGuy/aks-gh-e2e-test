package config

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateCommon(t *testing.T) {
	for _, tc := range []struct {
		name        string
		checker     CheckerConfig
		validateRes func(g *WithT, err error)
	}{
		// Valid checker
		{
			name: "valid-checker",
			checker: CheckerConfig{
				Name:     "checker",
				Type:     CheckerType("type"),
				Interval: 10,
				Timeout:  5,
			},
			validateRes: func(g *WithT, err error) {
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		// Missing name
		{
			name: "missing-name",
			checker: CheckerConfig{
				Name:     "",
				Type:     CheckerType("type"),
				Interval: 10,
				Timeout:  5,
			},
			validateRes: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("missing 'name'"))
			},
		},
		// Missing type
		{
			name: "missing-type",
			checker: CheckerConfig{
				Name:     "checker",
				Type:     "",
				Interval: 10,
				Timeout:  5,
			},
			validateRes: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("missing 'type'"))
			},
		},
		// Invalid interval
		{
			name: "invalid-interval",
			checker: CheckerConfig{
				Name:     "checker",
				Type:     CheckerType("type"),
				Interval: 0,
				Timeout:  5,
			},
			validateRes: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("invalid 'interval'"))
			},
		},
		// Invalid timeout
		{
			name: "invalid-timeout",
			checker: CheckerConfig{
				Name:     "checker",
				Type:     CheckerType("type"),
				Interval: 10,
				Timeout:  0,
			},
			validateRes: func(g *WithT, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("invalid 'timeout'"))
			},
		},
	} {
		g := NewWithT(t)
		err := tc.checker.ValidateCommon()
		t.Run(tc.name, func(t *testing.T) {
			tc.validateRes(g, err)
		})
	}
}
