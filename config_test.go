package main

import (
	"os"
	"testing"
	"time"
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

	t.Run("cache: true keeps caching enabled and parses cache_age", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/.bumpflow.yaml"
		os.WriteFile(path, []byte("cache: true\ncache_age: 3d\n"), 0644)
		cfg, found := loadConfigFile(path)
		if !found {
			t.Fatal("expected config file to be found")
		}
		if cfg.noCache {
			t.Error("expected noCache to be false")
		}
		if cfg.cacheAge != 3*24*time.Hour {
			t.Errorf("cacheAge = %v, want %v", cfg.cacheAge, 3*24*time.Hour)
		}
	})

	t.Run("cache: false disables caching", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/.bumpflow.yaml"
		os.WriteFile(path, []byte("cache: false\n"), 0644)
		cfg, found := loadConfigFile(path)
		if !found {
			t.Fatal("expected config file to be found")
		}
		if !cfg.noCache {
			t.Error("expected noCache to be true")
		}
	})

	t.Run("ignores invalid cache_age", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/.bumpflow.yaml"
		os.WriteFile(path, []byte("cache_age: not-a-duration\n"), 0644)
		cfg, found := loadConfigFile(path)
		if !found {
			t.Fatal("expected config file to be found")
		}
		if cfg.cacheAge != 0 {
			t.Errorf("cacheAge = %v, want 0", cfg.cacheAge)
		}
	})
}
