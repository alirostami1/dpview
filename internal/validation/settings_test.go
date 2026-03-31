package validation

import (
	"testing"
	"time"

	"codeberg.org/aros/dpview/internal/api"
)

func TestValidateSettingsPatchRejectsInvalidThemeValues(t *testing.T) {
	theme := "system"
	previewTheme := "solarized"

	if err := ValidateSettingsPatch(api.SettingsPatch{Theme: &theme}); err == nil {
		t.Fatalf("ValidateSettingsPatch() accepted invalid theme")
	}
	if err := ValidateSettingsPatch(api.SettingsPatch{PreviewTheme: &previewTheme}); err == nil {
		t.Fatalf("ValidateSettingsPatch() accepted invalid preview theme")
	}
}

func TestValidateSettingsPatchAcceptsKnownThemeValues(t *testing.T) {
	theme := "dark"
	previewTheme := "github"

	if err := ValidateSettingsPatch(api.SettingsPatch{Theme: &theme, PreviewTheme: &previewTheme}); err != nil {
		t.Fatalf("ValidateSettingsPatch() error = %v", err)
	}
}

func TestValidateRuntimeConfigRejectsInvalidValues(t *testing.T) {
	err := ValidateRuntimeConfig(RuntimeConfig{
		Port:          0,
		Theme:         "system",
		PreviewTheme:  "solarized",
		LogLevel:      "trace",
		MaxFileSize:   0,
		RenderTimeout: 0,
	})
	if err == nil {
		t.Fatalf("ValidateRuntimeConfig() accepted invalid config")
	}
}

func TestValidateRuntimeConfigAcceptsKnownValues(t *testing.T) {
	err := ValidateRuntimeConfig(RuntimeConfig{
		Port:          8090,
		Theme:         "light",
		PreviewTheme:  "default",
		LogLevel:      "info",
		MaxFileSize:   1,
		RenderTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("ValidateRuntimeConfig() error = %v", err)
	}
}
