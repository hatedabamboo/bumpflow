package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultCacheAge = 7 * 24 * time.Hour

type cachedTag struct {
	Tag  string `json:"tag"`
	SHA  string `json:"sha"`
	Date string `json:"date,omitempty"`
}

type cacheEntry struct {
	FetchedAt time.Time         `json:"fetched_at"`
	TagCount  int               `json:"tag_count"`
	Latest    cachedTag         `json:"latest"`
	TopTags   []cachedTag       `json:"top_tags"`
	Tags      map[string]string `json:"tags"`
}

type cacheFile struct {
	Repos map[string]cacheEntry `json:"repos"`
}

func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "bumpflow", "repos.json"), nil
}

func prepareCache(cfg config) *cacheFile {
	if cfg.noCache {
		return nil
	}
	return loadCache()
}

func loadCache() *cacheFile {
	empty := &cacheFile{Repos: map[string]cacheEntry{}}
	path, err := cachePath()
	if err != nil {
		slog.Debug("could not determine cache path", "error", err)
		return empty
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("cache file not found", "path", path)
		} else {
			slog.Debug("could not read cache file", "path", path, "error", err)
		}
		return empty
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		slog.Debug("could not parse cache file", "path", path, "error", err)
		return empty
	}
	if cf.Repos == nil {
		cf.Repos = map[string]cacheEntry{}
	}
	slog.Debug("cache file found", "path", path, "repos", len(cf.Repos))
	return &cf
}

func (cf *cacheFile) save() {
	path, err := cachePath()
	if err != nil {
		slog.Debug("could not determine cache path", "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		slog.Debug("could not create cache directory", "path", filepath.Dir(path), "error", err)
		return
	}
	data, err := json.Marshal(cf)
	if err != nil {
		slog.Debug("could not marshal cache", "error", err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		slog.Debug("could not write cache file", "path", path, "error", err)
		return
	}
	slog.Debug("wrote cache file", "path", path)
}

func (cf *cacheFile) get(ownerRepo string, count int, maxAge time.Duration) (*repoInfo, bool) {
	entry, ok := cf.Repos[ownerRepo]
	if !ok || entry.TagCount < count || len(entry.TopTags) == 0 {
		return nil, false
	}
	if time.Since(entry.FetchedAt) > maxAge {
		return nil, false
	}

	topTags := make([]tagInfo, min(count, len(entry.TopTags)))
	for i := range topTags {
		t := entry.TopTags[i]
		topTags[i] = tagInfo{tag: t.Tag, sha: t.SHA, date: t.Date}
	}

	return &repoInfo{
		latest:  tagInfo{tag: entry.Latest.Tag, sha: entry.Latest.SHA, date: entry.Latest.Date},
		topTags: topTags,
		tags:    entry.Tags,
	}, true
}

func (cf *cacheFile) set(ownerRepo string, info *repoInfo) {
	topTags := make([]cachedTag, len(info.topTags))
	for i, t := range info.topTags {
		topTags[i] = cachedTag{Tag: t.tag, SHA: t.sha, Date: t.date}
	}
	cf.Repos[ownerRepo] = cacheEntry{
		FetchedAt: time.Now(),
		TagCount:  len(info.topTags),
		Latest:    cachedTag{Tag: info.latest.tag, SHA: info.latest.sha, Date: info.latest.date},
		TopTags:   topTags,
		Tags:      info.tags,
	}
}

func parseCacheAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
