package config

import "fmt"

// ValidateCommon validates the common fields of a CheckerConfig.
func (c *CheckerConfig) ValidateCommon() error {
	if c.Name == "" {
		return fmt.Errorf("checker config missing 'name'")
	}
	if c.Type == "" {
		return fmt.Errorf("checker config missing 'type'")
	}
	if c.Interval < 0 {
		return fmt.Errorf("checker config invalid 'interval'")
	}
	if c.Timeout < 0 {
		return fmt.Errorf("checker config invalid 'timeout'")
	}
	return nil
}
