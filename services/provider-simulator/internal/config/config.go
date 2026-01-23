package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds global configuration for the simulator.
type Config struct {
	TargetURL string
	SecretKey string
	Verbose   bool
	DryRun    bool
}

// ScenarioFile represents a YAML scenario file.
type ScenarioFile struct {
	Name      string     `yaml:"name"`
	Target    string     `yaml:"target"`
	Scenarios []Scenario `yaml:"scenarios"`
}

// Scenario represents a test scenario with multiple steps.
type Scenario struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"`
	Steps    []Step `yaml:"steps"`
}

// Step represents a single webhook event in a scenario.
type Step struct {
	Event        string         `yaml:"event"`
	Delay        string         `yaml:"delay,omitempty"`
	Data         map[string]any `yaml:"data"`
	ExpectStatus int            `yaml:"expect_status"`
	SignOpts     *StepSignOpts  `yaml:"sign_opts,omitempty"`
}

// StepSignOpts allows per-step signature customization.
type StepSignOpts struct {
	SkipSignature   bool   `yaml:"skip_signature,omitempty"`
	InvalidKey      bool   `yaml:"invalid_key,omitempty"`
	TimestampOffset string `yaml:"timestamp_offset,omitempty"`
}

// GetDelay parses the delay string and returns a duration.
func (s *Step) GetDelay() (time.Duration, error) {
	if s.Delay == "" {
		return 0, nil
	}
	return time.ParseDuration(s.Delay)
}

// GetTimestampOffset parses the timestamp offset and returns a duration.
func (o *StepSignOpts) GetTimestampOffset() (time.Duration, error) {
	if o == nil || o.TimestampOffset == "" {
		return 0, nil
	}
	return time.ParseDuration(o.TimestampOffset)
}

// LoadScenarioFile loads a scenario file from the given path.
func LoadScenarioFile(path string) (*ScenarioFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading scenario file: %w", err)
	}

	var sf ScenarioFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing scenario file: %w", err)
	}

	return &sf, nil
}

// Validate checks if the scenario file is valid.
func (sf *ScenarioFile) Validate() error {
	if sf.Name == "" {
		return fmt.Errorf("scenario file must have a name")
	}
	if len(sf.Scenarios) == 0 {
		return fmt.Errorf("scenario file must have at least one scenario")
	}

	for i, s := range sf.Scenarios {
		if s.Name == "" {
			return fmt.Errorf("scenario %d must have a name", i)
		}
		if s.Provider == "" {
			return fmt.Errorf("scenario %q must have a provider", s.Name)
		}
		if !isValidProvider(s.Provider) {
			return fmt.Errorf("scenario %q has invalid provider %q", s.Name, s.Provider)
		}
		if len(s.Steps) == 0 {
			return fmt.Errorf("scenario %q must have at least one step", s.Name)
		}
		for j, step := range s.Steps {
			if step.Event == "" {
				return fmt.Errorf("scenario %q step %d must have an event", s.Name, j)
			}
		}
	}

	return nil
}

func isValidProvider(provider string) bool {
	switch provider {
	case "stripe", "adyen", "paypal":
		return true
	default:
		return false
	}
}
