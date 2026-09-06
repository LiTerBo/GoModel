package complexity

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/enterpilot/gomodel/config"
	"github.com/enterpilot/gomodel/ext"
)

func loadResult(t *testing.T, raw string) *config.LoadResult {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	return &config.LoadResult{Config: &config.Config{
		Extensions: map[string]yaml.Node{ConfigKey: node},
	}}
}

func TestInstallEnabled(t *testing.T) {
	t.Parallel()
	reg := &ext.Registry{}
	result := loadResult(t, `
enabled: true
virtual_model: smart
tiers:
  simple: ["lite"]
  complex: ["pro"]
thresholds:
  simple_medium: 0.4
  medium_complex: 0.6
  complex_very_complex: 0.85
`)
	if err := Install(reg, result); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if reg.RouteSelector() == nil {
		t.Fatal("selector not registered")
	}
	settings := reg.Settings()
	if len(settings) != 1 || settings[0].Descriptor().Key != "complexity_routing.thresholds" {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestInstallDisabledOrMissing(t *testing.T) {
	t.Parallel()
	reg := &ext.Registry{}
	if err := Install(reg, loadResult(t, "enabled: false\n")); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if reg.RouteSelector() != nil {
		t.Fatal("disabled section must not register a selector")
	}
	// A result without the section is a no-op, not an error.
	if err := Install(reg, &config.LoadResult{Config: &config.Config{}}); err != nil {
		t.Fatalf("Install(empty) error = %v", err)
	}
	if err := Install(reg, nil); err != nil {
		t.Fatalf("Install(nil) error = %v", err)
	}
}

func TestInstallRejectsUnknownKeys(t *testing.T) {
	t.Parallel()
	reg := &ext.Registry{}
	err := Install(reg, loadResult(t, "enabled: true\nstrange: 1\n"))
	if err == nil {
		t.Fatal("unknown key must fail (KnownFields strictness)")
	}
}
