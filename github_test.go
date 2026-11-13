package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGithubGet(t *testing.T) {
	t.Run("returns body on 200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"ok":true}`))
		}))
		defer srv.Close()

		data, err := githubGet(srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != `{"ok":true}` {
			t.Errorf("got %q, want %q", string(data), `{"ok":true}`)
		}
	})

	t.Run("rate limit error without token", func(t *testing.T) {
		orig := ghToken
		ghToken = ""
		defer func() { ghToken = orig }()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		_, err := githubGet(srv.URL)
		if err == nil {
			t.Fatal("expected error for 403 without token")
		}
	})

	t.Run("non-200 returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		_, err := githubGet(srv.URL)
		if err == nil {
			t.Fatal("expected error for 404")
		}
	})

	t.Run("401 returns authentication error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		_, err := githubGet(srv.URL)
		if err == nil {
			t.Fatal("expected error for 401")
		}
		if !strings.Contains(err.Error(), "invalid or has been revoked") {
			t.Errorf("expected revoked-token message, got: %v", err)
		}
	})

	t.Run("403 with rate limit header returns rate limit error", func(t *testing.T) {
		orig := ghToken
		ghToken = "sometoken"
		defer func() { ghToken = orig }()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		_, err := githubGet(srv.URL)
		if err == nil {
			t.Fatal("expected error for rate-limited 403")
		}
		if !strings.Contains(err.Error(), "rate limit exceeded") {
			t.Errorf("expected rate limit message, got: %v", err)
		}
	})

	t.Run("403 with token but no rate limit header returns forbidden error", func(t *testing.T) {
		orig := ghToken
		ghToken = "sometoken"
		defer func() { ghToken = orig }()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		_, err := githubGet(srv.URL)
		if err == nil {
			t.Fatal("expected error for forbidden 403 with token")
		}
		if !strings.Contains(err.Error(), "access forbidden") {
			t.Errorf("expected forbidden message, got: %v", err)
		}
	})

	t.Run("sends bumpflow user agent", func(t *testing.T) {
		orig := version
		version = "v9.9.9"
		defer func() { version = orig }()

		var gotUA string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUA = r.Header.Get("User-Agent")
			w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		githubGet(srv.URL)
		if gotUA != "bumpflow/v9.9.9" {
			t.Errorf("User-Agent = %q, want %q", gotUA, "bumpflow/v9.9.9")
		}
	})

	t.Run("sends auth header when token set", func(t *testing.T) {
		orig := ghToken
		ghToken = "mytoken"
		defer func() { ghToken = orig }()

		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		githubGet(srv.URL)
		if gotAuth != "Bearer mytoken" {
			t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer mytoken")
		}
	})
}

func TestGetCommitDate(t *testing.T) {
	t.Run("returns YYYY-MM-DD portion", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-15T10:30:00Z"}}}`))
		}))
		defer srv.Close()

		got := getCommitDate("owner/repo", "abc123", srv.URL)
		if got != "2024-01-15" {
			t.Errorf("got %q, want %q", got, "2024-01-15")
		}
	})

	t.Run("api error returns unknown", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		got := getCommitDate("owner/repo", "abc123", srv.URL)
		if got != "unknown" {
			t.Errorf("got %q, want %q", got, "unknown")
		}
	})

	t.Run("malformed json returns unknown", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not json`))
		}))
		defer srv.Close()

		got := getCommitDate("owner/repo", "abc123", srv.URL)
		if got != "unknown" {
			t.Errorf("got %q, want %q", got, "unknown")
		}
	})
}

func TestGetRepoTagInfo(t *testing.T) {
	t.Run("returns latest semver tag", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/commits/") {
				w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-15T00:00:00Z"}}}`))
				return
			}
			w.Write([]byte(`[
				{"name":"v1.0.0","commit":{"sha":"bbb"}},
				{"name":"v2.0.0","commit":{"sha":"aaa"}},
				{"name":"not-semver","commit":{"sha":"ccc"}}
			]`))
		}))
		defer srv.Close()

		info, err := getRepoTagInfo("owner/repo", srv.URL, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		if info.latest.tag != "v2.0.0" {
			t.Errorf("latest tag = %q, want v2.0.0", info.latest.tag)
		}
		if info.latest.sha != "aaa" {
			t.Errorf("latest sha = %q, want aaa", info.latest.sha)
		}
		if len(info.tags) != 3 {
			t.Errorf("tags count = %d, want 3", len(info.tags))
		}
	})

	t.Run("topTags sorted descending and limited by count", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/commits/") {
				w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-15T00:00:00Z"}}}`))
				return
			}
			w.Write([]byte(`[
				{"name":"v1.0.0","commit":{"sha":"aaa"}},
				{"name":"v3.0.0","commit":{"sha":"ccc"}},
				{"name":"v2.0.0","commit":{"sha":"bbb"}}
			]`))
		}))
		defer srv.Close()

		info, err := getRepoTagInfo("owner/repo", srv.URL, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.topTags) != 2 {
			t.Fatalf("topTags count = %d, want 2", len(info.topTags))
		}
		if info.topTags[0].tag != "v3.0.0" {
			t.Errorf("topTags[0] = %q, want v3.0.0", info.topTags[0].tag)
		}
		if info.topTags[1].tag != "v2.0.0" {
			t.Errorf("topTags[1] = %q, want v2.0.0", info.topTags[1].tag)
		}
	})

	t.Run("count larger than available tags is clamped", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/commits/") {
				w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-15T00:00:00Z"}}}`))
				return
			}
			w.Write([]byte(`[
				{"name":"v1.0.0","commit":{"sha":"aaa"}},
				{"name":"v2.0.0","commit":{"sha":"bbb"}}
			]`))
		}))
		defer srv.Close()

		info, err := getRepoTagInfo("owner/repo", srv.URL, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.topTags) != 2 {
			t.Errorf("topTags count = %d, want 2", len(info.topTags))
		}
	})

	t.Run("no semver tags returns nil info", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"name":"latest","commit":{"sha":"aaa"}}]`))
		}))
		defer srv.Close()

		info, err := getRepoTagInfo("owner/repo", srv.URL, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info != nil {
			t.Errorf("expected nil info, got %+v", info)
		}
	})

	t.Run("api error propagates", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := getRepoTagInfo("owner/repo", srv.URL, 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestNextLink(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty", "", ""},
		{"next and last", `<https://api.github.com/repositories/1/tags?per_page=100&page=2>; rel="next", <https://api.github.com/repositories/1/tags?per_page=100&page=6>; rel="last"`, "https://api.github.com/repositories/1/tags?per_page=100&page=2"},
		{"next listed second", `<https://x/?page=1>; rel="prev", <https://x/?page=3>; rel="next"`, "https://x/?page=3"},
		{"last page has no next", `<https://x/?page=1>; rel="first", <https://x/?page=2>; rel="prev"`, ""},
		{"malformed url", `https://x/?page=2; rel="next"`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextLink(tt.header); got != tt.want {
				t.Errorf("nextLink(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestGetRepoTagInfoPagination(t *testing.T) {
	newServer := func(pages map[string]string, nextOf func(page string, srvURL string) string, hits *atomic.Int32) *httptest.Server {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/commits/") {
				w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-15T00:00:00Z"}}}`))
				return
			}
			hits.Add(1)
			page := r.URL.Query().Get("page")
			if page == "" {
				page = "1"
			}
			if next := nextOf(page, srv.URL); next != "" {
				w.Header().Set("Link", `<`+next+`>; rel="next"`)
			}
			w.Write([]byte(pages[page]))
		}))
		return srv
	}

	tests := []struct {
		name       string
		pages      map[string]string
		nextOf     func(page, srvURL string) string
		wantHits   int32
		wantLatest string
		wantTags   []string
	}{
		{
			name: "follows three pages",
			pages: map[string]string{
				"1": `[{"name":"v1.0.0","commit":{"sha":"a1"}},{"name":"v1","commit":{"sha":"a1"}}]`,
				"2": `[{"name":"v2.0.0","commit":{"sha":"b2"}}]`,
				"3": `[{"name":"v3.0.0","commit":{"sha":"c3"}}]`,
			},
			nextOf: func(page, srvURL string) string {
				switch page {
				case "1":
					return srvURL + "/repos/owner/repo/tags?per_page=100&page=2"
				case "2":
					return srvURL + "/repos/owner/repo/tags?per_page=100&page=3"
				}
				return ""
			},
			wantHits:   3,
			wantLatest: "v3.0.0",
			wantTags:   []string{"v1", "v1.0.0", "v2.0.0", "v3.0.0"},
		},
		{
			name:  "stops at page cap",
			pages: map[string]string{"1": `[{"name":"v1.0.0","commit":{"sha":"a1"}}]`},
			nextOf: func(page, srvURL string) string {
				return srvURL + "/repos/owner/repo/tags?per_page=100&page=1"
			},
			wantHits:   maxTagPages,
			wantLatest: "v1.0.0",
			wantTags:   []string{"v1.0.0"},
		},
		{
			name: "ignores next link to another host",
			pages: map[string]string{
				"1": `[{"name":"v1.0.0","commit":{"sha":"a1"}}]`,
			},
			nextOf: func(page, srvURL string) string {
				return "https://evil.example.com/repos/owner/repo/tags?page=2"
			},
			wantHits:   1,
			wantLatest: "v1.0.0",
			wantTags:   []string{"v1.0.0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			srv := newServer(tt.pages, tt.nextOf, &hits)
			defer srv.Close()

			info, err := getRepoTagInfo("owner/repo", srv.URL, 10)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hits.Load() != tt.wantHits {
				t.Errorf("tag page requests = %d, want %d", hits.Load(), tt.wantHits)
			}
			if info.latest.tag != tt.wantLatest {
				t.Errorf("latest = %q, want %q", info.latest.tag, tt.wantLatest)
			}
			for _, tag := range tt.wantTags {
				if _, ok := info.tags[tag]; !ok {
					t.Errorf("missing tag %q in %v", tag, info.tags)
				}
			}
		})
	}

	t.Run("error on later page propagates", func(t *testing.T) {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "2" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Link", `<`+srv.URL+`/repos/owner/repo/tags?page=2>; rel="next"`)
			w.Write([]byte(`[{"name":"v1.0.0","commit":{"sha":"a1"}}]`))
		}))
		defer srv.Close()

		if _, err := getRepoTagInfo("owner/repo", srv.URL, 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("sha resolves to tag found only on a later page", func(t *testing.T) {
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/commits/") {
				w.Write([]byte(`{}`))
				return
			}
			if r.URL.Query().Get("page") == "2" {
				w.Write([]byte(`[{"name":"v0.9.0","commit":{"sha":"old"}}]`))
				return
			}
			w.Header().Set("Link", `<`+srv.URL+`/repos/owner/repo/tags?page=2>; rel="next"`)
			w.Write([]byte(`[{"name":"v1.0.0","commit":{"sha":"new"}}]`))
		}))
		defer srv.Close()

		info, err := getRepoTagInfo("owner/repo", srv.URL, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := bestTagForSHA(info, "old"); got != "v0.9.0" {
			t.Errorf("bestTagForSHA = %q, want v0.9.0", got)
		}
	})
}

func TestBestTagForSHA(t *testing.T) {
	info := &repoInfo{
		tags: map[string]string{
			"v1.0.0":     "sha1",
			"v1.5.0":     "sha1",
			"not-semver": "sha1",
			"v2.0.0":     "sha2",
		},
	}

	t.Run("returns highest semver tag for sha", func(t *testing.T) {
		got := bestTagForSHA(info, "sha1")
		if got != "v1.5.0" {
			t.Errorf("got %q, want v1.5.0", got)
		}
	})

	t.Run("returns empty for unknown sha", func(t *testing.T) {
		got := bestTagForSHA(info, "unknown")
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("prefers precise version over short alias", func(t *testing.T) {
		info2 := &repoInfo{
			tags: map[string]string{
				"v9":      "abc",
				"v9.0.0":  "abc",
				"v10.0.0": "def",
			},
		}
		got := bestTagForSHA(info2, "abc")
		if got != "v9.0.0" {
			t.Errorf("got %q, want v9.0.0", got)
		}
	})

	t.Run("prefers higher version regardless of precision", func(t *testing.T) {
		info2 := &repoInfo{
			tags: map[string]string{
				"v2.0":   "abc",
				"v3.0.0": "abc",
			},
		}
		got := bestTagForSHA(info2, "abc")
		if got != "v3.0.0" {
			t.Errorf("got %q, want v3.0.0", got)
		}
	})
}

func TestFetchRepos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/commits/") {
			w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-01T00:00:00Z"}}}`))
			return
		}
		w.Write([]byte(`[{"name":"v1.0.0","commit":{"sha":"abc1234"}}]`))
	}))
	defer srv.Close()

	checked, errs := fetchRepos([]string{"owner/repo-a", "owner/repo-b"}, 10, srv.URL, nil, 0)

	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(checked) != 2 {
		t.Errorf("checked count = %d, want 2", len(checked))
	}
	for _, repo := range []string{"owner/repo-a", "owner/repo-b"} {
		if checked[repo] == nil {
			t.Errorf("missing info for %s", repo)
		}
	}
}

func TestFetchReposUsesCache(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if strings.Contains(r.URL.Path, "/commits/") {
			w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-01T00:00:00Z"}}}`))
			return
		}
		w.Write([]byte(`[{"name":"v1.0.0","commit":{"sha":"abc1234"}}]`))
	}))
	defer srv.Close()

	t.Run("fresh cache is served without hitting the API", func(t *testing.T) {
		cache := &cacheFile{Repos: map[string]cacheEntry{}}
		cache.set("owner/repo", &repoInfo{
			latest:  tagInfo{tag: "v1.0.0", sha: "abc1234", date: "2024-01-01"},
			topTags: []tagInfo{{tag: "v1.0.0", sha: "abc1234", date: "2024-01-01"}},
			tags:    map[string]string{"v1.0.0": "abc1234"},
		})

		hits.Store(0)
		checked, errs := fetchRepos([]string{"owner/repo"}, 1, srv.URL, cache, defaultCacheAge)
		if len(errs) != 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
		if hits.Load() != 0 {
			t.Errorf("expected 0 API hits, got %d", hits.Load())
		}
		if checked["owner/repo"] == nil || checked["owner/repo"].latest.tag != "v1.0.0" {
			t.Errorf("expected cached info to be served, got %+v", checked["owner/repo"])
		}
	})

	t.Run("stale cache falls through to the API", func(t *testing.T) {
		cache := &cacheFile{Repos: map[string]cacheEntry{}}
		cache.set("owner/repo", &repoInfo{
			latest:  tagInfo{tag: "v0.0.1", sha: "old", date: "2020-01-01"},
			topTags: []tagInfo{{tag: "v0.0.1", sha: "old", date: "2020-01-01"}},
			tags:    map[string]string{"v0.0.1": "old"},
		})
		entry := cache.Repos["owner/repo"]
		entry.FetchedAt = time.Now().Add(-8 * 24 * time.Hour)
		cache.Repos["owner/repo"] = entry

		hits.Store(0)
		checked, errs := fetchRepos([]string{"owner/repo"}, 1, srv.URL, cache, defaultCacheAge)
		if len(errs) != 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
		if hits.Load() == 0 {
			t.Error("expected stale cache to trigger an API call")
		}
		if checked["owner/repo"] == nil || checked["owner/repo"].latest.tag != "v1.0.0" {
			t.Errorf("expected fresh info from API, got %+v", checked["owner/repo"])
		}
		if cache.Repos["owner/repo"].Latest.Tag != "v1.0.0" {
			t.Errorf("expected cache to be refreshed, got %+v", cache.Repos["owner/repo"])
		}
	})
}

func TestGetRepoTagInfoSkipsCommitDate(t *testing.T) {
	var commitHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/commits/") {
			commitHits.Add(1)
			w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-15T00:00:00Z"}}}`))
			return
		}
		w.Write([]byte(`[{"name":"v1.0.0","commit":{"sha":"aaa"}}]`))
	}))
	defer srv.Close()

	info, err := getRepoTagInfo("owner/repo", srv.URL, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commitHits.Load() != 0 {
		t.Errorf("commit date requests = %d, want 0", commitHits.Load())
	}
	if info.latest.date != "" {
		t.Errorf("latest date = %q, want empty", info.latest.date)
	}
}

func TestFillLatestDates(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		cachedDate    string
		wantDate      string
		wantHits      int32
		wantCacheDate string
	}{
		{"fetches missing date once per repo", http.StatusOK, "", "2024-01-15", 1, "2024-01-15"},
		{"keeps known date without fetching", http.StatusOK, "2023-05-05", "2023-05-05", 0, "2023-05-05"},
		{"unknown on error is not cached", http.StatusNotFound, "", "unknown", 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				w.WriteHeader(tt.status)
				w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-15T00:00:00Z"}}}`))
			}))
			defer srv.Close()

			info := &repoInfo{
				latest:  tagInfo{tag: "v2.0.0", sha: "aaa", date: tt.cachedDate},
				topTags: []tagInfo{{tag: "v2.0.0", sha: "aaa", date: tt.cachedDate}},
				tags:    map[string]string{"v2.0.0": "aaa"},
			}
			cache := &cacheFile{Repos: map[string]cacheEntry{}}
			cache.set("owner/repo", info)
			fetchedAt := time.Now().Add(-time.Hour)
			entry := cache.Repos["owner/repo"]
			entry.FetchedAt = fetchedAt
			cache.Repos["owner/repo"] = entry

			checked := map[string]*repoInfo{"owner/repo": info, "other/repo": nil}
			actions := []action{
				{actionRef: "owner/repo", ownerRepo: "owner/repo"},
				{actionRef: "owner/repo/sub", ownerRepo: "owner/repo"},
				{actionRef: "other/repo", ownerRepo: "other/repo"},
			}
			fillLatestDates(actions, checked, srv.URL, cache)

			if hits.Load() != tt.wantHits {
				t.Errorf("API hits = %d, want %d", hits.Load(), tt.wantHits)
			}
			for _, a := range actions[:2] {
				if a.latestDate != tt.wantDate {
					t.Errorf("%s latestDate = %q, want %q", a.actionRef, a.latestDate, tt.wantDate)
				}
			}
			if info.topTags[0].date != tt.wantDate {
				t.Errorf("topTags[0].date = %q, want %q", info.topTags[0].date, tt.wantDate)
			}
			got := cache.Repos["owner/repo"]
			if got.Latest.Date != tt.wantCacheDate || got.TopTags[0].Date != tt.wantCacheDate {
				t.Errorf("cached dates = (%q, %q), want %q", got.Latest.Date, got.TopTags[0].Date, tt.wantCacheDate)
			}
			if !got.FetchedAt.Equal(fetchedAt) {
				t.Errorf("FetchedAt changed: got %v, want %v", got.FetchedAt, fetchedAt)
			}
		})
	}
}
