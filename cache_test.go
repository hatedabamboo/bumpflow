package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseCacheAge(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"0d", 0, false},
		{"24h", 24 * time.Hour, false},
		{"90m", 90 * time.Minute, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-1d", 0, true},
	}
	for _, tt := range tests {
		got, err := parseCacheAge(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseCacheAge(%q) = nil error, want error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseCacheAge(%q) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseCacheAge(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestCacheGetSet(t *testing.T) {
	cf := &cacheFile{Repos: map[string]cacheEntry{}}
	info := &repoInfo{
		latest: tagInfo{tag: "v2.0.0", sha: "sha2", date: "2026-01-01"},
		topTags: []tagInfo{
			{tag: "v2.0.0", sha: "sha2", date: "2026-01-01"},
			{tag: "v1.0.0", sha: "sha1"},
		},
		tags: map[string]string{"v2.0.0": "sha2", "v1.0.0": "sha1"},
	}

	t.Run("miss when repo absent", func(t *testing.T) {
		_, ok := cf.get("owner/repo", 2, defaultCacheAge)
		if ok {
			t.Error("expected cache miss for absent repo")
		}
	})

	cf.set("owner/repo", info)

	t.Run("hit returns stored data", func(t *testing.T) {
		got, ok := cf.get("owner/repo", 2, defaultCacheAge)
		if !ok {
			t.Fatal("expected cache hit")
		}
		if got.latest.tag != "v2.0.0" || got.latest.sha != "sha2" {
			t.Errorf("latest = %+v, want v2.0.0/sha2", got.latest)
		}
		if len(got.topTags) != 2 {
			t.Errorf("topTags len = %d, want 2", len(got.topTags))
		}
		if got.tags["v1.0.0"] != "sha1" {
			t.Errorf("tags map not preserved: %+v", got.tags)
		}
	})

	t.Run("miss when requesting more tags than cached", func(t *testing.T) {
		_, ok := cf.get("owner/repo", 5, defaultCacheAge)
		if ok {
			t.Error("expected cache miss when count exceeds cached tag count")
		}
	})

	t.Run("hit when requesting fewer tags than cached", func(t *testing.T) {
		got, ok := cf.get("owner/repo", 1, defaultCacheAge)
		if !ok {
			t.Fatal("expected cache hit")
		}
		if len(got.topTags) != 1 {
			t.Errorf("topTags len = %d, want 1", len(got.topTags))
		}
	})

	t.Run("miss when entry older than max age", func(t *testing.T) {
		entry := cf.Repos["owner/repo"]
		entry.FetchedAt = time.Now().Add(-8 * 24 * time.Hour)
		cf.Repos["owner/repo"] = entry

		_, ok := cf.get("owner/repo", 2, defaultCacheAge)
		if ok {
			t.Error("expected cache miss for stale entry")
		}
	})
}

func TestCacheLoadSaveRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cf := loadCache()
	if len(cf.Repos) != 0 {
		t.Fatalf("expected empty cache, got %+v", cf.Repos)
	}

	info := &repoInfo{
		latest:  tagInfo{tag: "v1.0.0", sha: "abc123", date: "2026-01-01"},
		topTags: []tagInfo{{tag: "v1.0.0", sha: "abc123", date: "2026-01-01"}},
		tags:    map[string]string{"v1.0.0": "abc123"},
	}
	cf.set("owner/repo", info)
	cf.save()

	path := filepath.Join(home, ".cache", "bumpflow", "repos.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected cache file at %s: %v", path, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("cache file is not valid JSON: %v", err)
	}

	reloaded := loadCache()
	got, ok := reloaded.get("owner/repo", 1, defaultCacheAge)
	if !ok {
		t.Fatal("expected cache hit after reload")
	}
	if got.latest.tag != "v1.0.0" || got.latest.sha != "abc123" {
		t.Errorf("latest = %+v, want v1.0.0/abc123", got.latest)
	}
}

func TestPrepareCache(t *testing.T) {
	t.Run("nil when disabled", func(t *testing.T) {
		if c := prepareCache(config{noCache: true}); c != nil {
			t.Error("expected nil cache when noCache is true")
		}
	})

	t.Run("non-nil by default", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		if c := prepareCache(config{noCache: false}); c == nil {
			t.Error("expected non-nil cache when noCache is false")
		}
	})
}
