package validation

import (
	"fmt"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"github.com/go-playground/validator/v10"
)

const (
	resolvedThemeRule = "required,oneof=light dark"
	previewThemeRule  = "required,oneof=default github notion paper"
	logLevelRule      = "required,oneof=debug info error"
	portRule          = "min=1,max=65535"
	positiveIntRule   = "gt=0"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type RuntimeConfig struct {
	Port          int
	Theme         string
	PreviewTheme  string
	LogLevel      string
	MaxFileSize   int64
	RenderTimeout time.Duration
}

func ValidateSettingsPatch(patch api.SettingsPatch) error {
	if patch.Theme != nil {
		if err := validateVar("theme", *patch.Theme, resolvedThemeRule); err != nil {
			return err
		}
	}
	if patch.PreviewTheme != nil {
		if err := validateVar("preview theme", *patch.PreviewTheme, previewThemeRule); err != nil {
			return err
		}
	}
	return nil
}

func ValidateRuntimeConfig(cfg RuntimeConfig) error {
	if err := validateVar("port", cfg.Port, portRule); err != nil {
		return err
	}
	if err := validateVar("theme", cfg.Theme, resolvedThemeRule); err != nil {
		return err
	}
	if err := validateVar("preview theme", cfg.PreviewTheme, previewThemeRule); err != nil {
		return err
	}
	if err := validateVar("log level", cfg.LogLevel, logLevelRule); err != nil {
		return err
	}
	if err := validateVar("max file size", cfg.MaxFileSize, positiveIntRule); err != nil {
		return err
	}
	if err := validateVar("render timeout", cfg.RenderTimeout, positiveIntRule); err != nil {
		return err
	}
	return nil
}

func validateVar(label string, value any, rule string) error {
	err := validate.Var(value, rule)
	if err == nil {
		return nil
	}

	switch label {
	case "theme":
		return fmt.Errorf("invalid theme %q", value)
	case "preview theme":
		return fmt.Errorf("invalid preview theme %q", value)
	case "log level":
		return fmt.Errorf("invalid log level %q", value)
	case "port":
		return fmt.Errorf("invalid port %d", value)
	case "max file size":
		return fmt.Errorf("invalid max file size %d", value)
	case "render timeout":
		return fmt.Errorf("invalid render timeout %s", value)
	default:
		return err
	}
}
