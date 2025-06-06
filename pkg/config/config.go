package config

type CheckerType string

const (
	CheckTypeDNS        CheckerType = "dns"
	CheckTypePodStartup CheckerType = "podStartup"
)

type Config struct {
	Checkers []CheckerConfig `yaml:"checkers"`
}

type CheckerConfig struct {
	// the unique name of the checker, used to identify the checker in the system
	Name string `yaml:"name"`

	// the type of the checker, used to determine which checker implementation to use
	// each checker type must have a corresponding builder registered in the system
	// each checker type must has a Profile configuration
	Type CheckerType `yaml:"type"`

	// the interval in seconds at which the checker should run
	Interval int `yaml:"interval"`

	// the profile configuration for the DNS checker, this field is required if Type is CheckTypeDNS
	DNSProfile *DNSProfile `yaml:"dnsProfile,omitempty"`

	// the profile configuration for the Pod startup checker, this field is required if Type is CheckTypePodStartup
	PodStartupProfile *PodStartupProfile `yaml:"podStartupProfile,omitempty"`
}

type DNSProfile struct {
	Domain string `yaml:"domain"` // example field for DNS profile

	// TODO: add more fields for DNS profile configuration
}

type PodStartupProfile struct {
	Namespace string `yaml:"namespace"` // example field for Pod startup profile
	PodName   string `yaml:"podName"`   // example field for Pod startup profile
	// TODO: add more fields for Pod startup profile configuration
}
