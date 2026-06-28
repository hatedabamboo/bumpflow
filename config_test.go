package main

import (
	"os"
	"testing"
)

func TestLoadConfigFile(t *testing.T) {
	t.Run("strips double quotes from values", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/.bumpflow.yaml"
		os.WriteFile(path, []byte(`target_file: "my-workflow.yml"
target_action: "actions/checkout"
always_sha: true`), 0644)
		cfg, found := loadConfigFile(path)
		if !found {
			t.Fatal("expected config file to be found")
		}
		if cfg.targetFile != "my-workflow.yml" {
			t.Errorf("targetFile = %q, want %q", cfg.targetFile, "my-workflow.yml")
		}
		if cfg.targetAction != "actions/checkout" {
			t.Errorf("targetAction = %q, want %q", cfg.targetAction, "actions/checkout")
		}
		if !cfg.useHash {
			t.Error("expected useHash to be true")
		}
	})

	t.Run("strips single quotes from values", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/.bumpflow.yaml"
		os.WriteFile(path, []byte("target_file: 'ci.yml'\n"), 0644)
		cfg, found := loadConfigFile(path)
		if !found {
			t.Fatal("expected config file to be found")
		}
		if cfg.targetFile != "ci.yml" {
			t.Errorf("targetFile = %q, want %q", cfg.targetFile, "ci.yml")
		}
	})

	t.Run("preserves # inside quoted values", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/.bumpflow.yaml"
		os.WriteFile(path, []byte(`target_action: "owner/repo#ref"`+"\n"), 0644)
		cfg, found := loadConfigFile(path)
		if !found {
			t.Fatal("expected config file to be found")
		}
		if cfg.targetAction != "owner/repo#ref" {
			t.Errorf("targetAction = %q, want %q", cfg.targetAction, "owner/repo#ref")
		}
	})

	t.Run("strips inline comment from unquoted values", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/.bumpflow.yaml"
		os.WriteFile(path, []byte("always_sha: true # enable sha pinning\n"), 0644)
		cfg, found := loadConfigFile(path)
		if !found {
			t.Fatal("expected config file to be found")
		}
		if !cfg.useHash {
			t.Error("expected useHash to be true")
		}
	})

	t.Run("no config file returns false", func(t *testing.T) {
		dir := t.TempDir()
		_, found := loadConfigFile(dir + "/nonexistent.yaml")
		if found {
			t.Error("expected found to be false")
		}
	})
}
