package config

import (
	"strings"
	"testing"
)

func TestValidate_ValidAspectRatios(t *testing.T) {
	validRatios := []string{"16:9", "4:3", "16:10"}

	for _, ratio := range validRatios {
		cfg := DefaultConfig()
		cfg.AspectRatio = ratio

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() returned error for valid aspectRatio %q: %v", ratio, err)
		}
	}
}

func TestValidate_InvalidAspectRatio(t *testing.T) {
	invalidRatios := []string{"16:10:1", "1:1", "21:9", "invalid", "16-9"}

	for _, ratio := range invalidRatios {
		cfg := DefaultConfig()
		cfg.AspectRatio = ratio

		err := cfg.Validate()
		if err == nil {
			t.Errorf("Validate() should return error for invalid aspectRatio %q", ratio)
			continue
		}

		if !strings.Contains(err.Error(), "aspectRatio") {
			t.Errorf("error message should mention aspectRatio, got: %v", err)
		}
		if !strings.Contains(err.Error(), ratio) {
			t.Errorf("error message should include the invalid value %q, got: %v", ratio, err)
		}
	}
}

func TestValidate_ValidTransitions(t *testing.T) {
	validTransitions := []string{"none", "fade", "slide", "push", "zoom"}

	for _, transition := range validTransitions {
		cfg := DefaultConfig()
		cfg.Transition = transition

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() returned error for valid transition %q: %v", transition, err)
		}
	}
}

func TestValidate_InvalidTransition(t *testing.T) {
	invalidTransitions := []string{"dissolve", "wipe", "flip", "invalid", "FADE"}

	for _, transition := range invalidTransitions {
		cfg := DefaultConfig()
		cfg.Transition = transition

		err := cfg.Validate()
		if err == nil {
			t.Errorf("Validate() should return error for invalid transition %q", transition)
			continue
		}

		if !strings.Contains(err.Error(), "transition") {
			t.Errorf("error message should mention transition, got: %v", err)
		}
		if !strings.Contains(err.Error(), transition) {
			t.Errorf("error message should include the invalid value %q, got: %v", transition, err)
		}
	}
}

func TestValidate_EmptyValues(t *testing.T) {
	cfg := &Config{
		AspectRatio: "",
		Transition:  "",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow empty values (use defaults), got error: %v", err)
	}
}

func TestValidate_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if err := cfg.Validate(); err != nil {
		t.Errorf("DefaultConfig() should pass validation, got error: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	// When both aspectRatio and transition are invalid, the first error should be returned
	cfg := &Config{
		AspectRatio: "invalid-ratio",
		Transition:  "invalid-transition",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should return error for invalid config")
	}

	// First validation check is aspectRatio
	if !strings.Contains(err.Error(), "aspectRatio") {
		t.Errorf("error should mention aspectRatio first, got: %v", err)
	}
}
