package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	t.Setenv("GEMQUERY_PROJECT", "")

	cfg, err := Load("/nonexistent/path.toml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GCP.Project != "test-project" {
		t.Errorf("project = %q, want test-project", cfg.GCP.Project)
	}
	if cfg.GCP.Location != "us-central1" {
		t.Errorf("location = %q, want us-central1", cfg.GCP.Location)
	}
	if cfg.Model.Name != "gemini-2.5-flash" {
		t.Errorf("model = %q, want gemini-2.5-flash", cfg.Model.Name)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("GEMQUERY_PROJECT", "env-project")
	t.Setenv("GEMQUERY_LOCATION", "asia-northeast1")
	t.Setenv("GEMQUERY_MODEL", "gemini-2.5-pro")

	cfg, err := Load("/nonexistent/path.toml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GCP.Project != "env-project" {
		t.Errorf("project = %q, want env-project", cfg.GCP.Project)
	}
	if cfg.GCP.Location != "asia-northeast1" {
		t.Errorf("location = %q, want asia-northeast1", cfg.GCP.Location)
	}
	if cfg.Model.Name != "gemini-2.5-pro" {
		t.Errorf("model = %q, want gemini-2.5-pro", cfg.Model.Name)
	}
}

func TestLoad_TOMLFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `[gcp]
project  = "toml-project"
location = "europe-west1"

[model]
name = "custom-model"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GEMQUERY_PROJECT", "")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GCP.Project != "toml-project" {
		t.Errorf("project = %q, want toml-project", cfg.GCP.Project)
	}
	if cfg.GCP.Location != "europe-west1" {
		t.Errorf("location = %q, want europe-west1", cfg.GCP.Location)
	}
	if cfg.Model.Name != "custom-model" {
		t.Errorf("model = %q, want custom-model", cfg.Model.Name)
	}
}

func TestLoad_EnvOverridesToml(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `[gcp]
project = "toml-project"
[model]
name = "toml-model"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GEMQUERY_PROJECT", "")
	t.Setenv("GEMQUERY_MODEL", "env-model")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model.Name != "env-model" {
		t.Errorf("model = %q, want env-model (env override)", cfg.Model.Name)
	}
	if cfg.GCP.Project != "toml-project" {
		t.Errorf("project = %q, want toml-project", cfg.GCP.Project)
	}
}

func TestLoad_MissingProject(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GEMQUERY_PROJECT", "")

	_, err := Load("/nonexistent/path.toml")
	if err == nil {
		t.Error("expected error when project is not set")
	}
}

func TestApplyFlags(t *testing.T) {
	cfg := &Config{Model: ModelConfig{Name: "original"}}
	cfg.ApplyFlags("override-model", "")
	if cfg.Model.Name != "override-model" {
		t.Errorf("model = %q, want override-model", cfg.Model.Name)
	}
}

func TestApplyFlags_Empty(t *testing.T) {
	cfg := &Config{Model: ModelConfig{Name: "original"}}
	cfg.ApplyFlags("", "")
	if cfg.Model.Name != "original" {
		t.Errorf("model = %q, want original (empty flag should not override)", cfg.Model.Name)
	}
}

func TestApplyFlags_JvizPath(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyFlags("", "/usr/local/bin/jviz")
	if cfg.Tools.JvizPath != "/usr/local/bin/jviz" {
		t.Errorf("jviz_path = %q, want /usr/local/bin/jviz", cfg.Tools.JvizPath)
	}
}

func TestLoad_JvizPathEnv(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	t.Setenv("GEMQUERY_JVIZ_PATH", "/opt/bin/jviz")

	cfg, err := Load("/nonexistent/path.toml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Tools.JvizPath != "/opt/bin/jviz" {
		t.Errorf("jviz_path = %q, want /opt/bin/jviz", cfg.Tools.JvizPath)
	}
}
