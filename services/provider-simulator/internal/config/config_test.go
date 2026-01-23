package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadScenarioFile(t *testing.T) {
	// Create a temporary scenario file
	content := `name: "Test Scenario"
target: "http://localhost:8080"

scenarios:
  - name: "test_scenario"
    provider: "stripe"
    steps:
      - event: "payment_intent.succeeded"
        data:
          payment_id: "pi_test"
          amount: 10000
        expect_status: 200
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_scenario.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	t.Run("load valid file", func(t *testing.T) {
		sf, err := LoadScenarioFile(tmpFile)
		if err != nil {
			t.Errorf("LoadScenarioFile() error = %v", err)
		}

		if sf.Name != "Test Scenario" {
			t.Errorf("Name = %v, want 'Test Scenario'", sf.Name)
		}

		if sf.Target != "http://localhost:8080" {
			t.Errorf("Target = %v, want 'http://localhost:8080'", sf.Target)
		}

		if len(sf.Scenarios) != 1 {
			t.Errorf("len(Scenarios) = %d, want 1", len(sf.Scenarios))
		}
	})

	t.Run("load nonexistent file", func(t *testing.T) {
		_, err := LoadScenarioFile("/nonexistent/file.yaml")
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})
}

func TestScenarioFile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		sf      ScenarioFile
		wantErr bool
	}{
		{
			name: "valid scenario",
			sf: ScenarioFile{
				Name: "Test",
				Scenarios: []Scenario{
					{
						Name:     "test",
						Provider: "stripe",
						Steps: []Step{
							{Event: "payment_intent.succeeded"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			sf: ScenarioFile{
				Scenarios: []Scenario{
					{Name: "test", Provider: "stripe", Steps: []Step{{Event: "test"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "no scenarios",
			sf: ScenarioFile{
				Name:      "Test",
				Scenarios: []Scenario{},
			},
			wantErr: true,
		},
		{
			name: "invalid provider",
			sf: ScenarioFile{
				Name: "Test",
				Scenarios: []Scenario{
					{
						Name:     "test",
						Provider: "invalid",
						Steps:    []Step{{Event: "test"}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing event",
			sf: ScenarioFile{
				Name: "Test",
				Scenarios: []Scenario{
					{
						Name:     "test",
						Provider: "stripe",
						Steps:    []Step{{Event: ""}},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sf.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStep_GetDelay(t *testing.T) {
	tests := []struct {
		name    string
		delay   string
		wantErr bool
	}{
		{"empty delay", "", false},
		{"milliseconds", "500ms", false},
		{"seconds", "2s", false},
		{"minutes", "1m", false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Step{Delay: tt.delay}
			_, err := s.GetDelay()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDelay() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeclineCodes(t *testing.T) {
	t.Run("stripe decline codes", func(t *testing.T) {
		code, ok := StripeDeclineCodes["insufficient_funds"]
		if !ok {
			t.Error("Expected insufficient_funds code")
		}
		if code.Type != DeclineTypeSoft {
			t.Errorf("Type = %v, want SOFT", code.Type)
		}
	})

	t.Run("adyen decline codes", func(t *testing.T) {
		code, ok := AdyenDeclineCodes["Refused:51"]
		if !ok {
			t.Error("Expected Refused:51 code")
		}
		if code.Canonical != "INSUFFICIENT_FUNDS" {
			t.Errorf("Canonical = %v, want INSUFFICIENT_FUNDS", code.Canonical)
		}
	})

	t.Run("paypal decline codes", func(t *testing.T) {
		code, ok := PayPalDeclineCodes["CREDIT_CARD_EXPIRED"]
		if !ok {
			t.Error("Expected CREDIT_CARD_EXPIRED code")
		}
		if code.Type != DeclineTypeHard {
			t.Errorf("Type = %v, want HARD", code.Type)
		}
	})
}
