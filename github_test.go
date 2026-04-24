package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	orig := apiBaseURL
	defer func() { apiBaseURL = orig }()

	t.Run("returns YYYY-MM-DD portion", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-15T10:30:00Z"}}}`))
		}))
		defer srv.Close()
		apiBaseURL = srv.URL

		got := getCommitDate("owner/repo", "abc123")
		if got != "2024-01-15" {
			t.Errorf("got %q, want %q", got, "2024-01-15")
		}
	})

	t.Run("api error returns unknown", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		apiBaseURL = srv.URL

		got := getCommitDate("owner/repo", "abc123")
		if got != "unknown" {
			t.Errorf("got %q, want %q", got, "unknown")
		}
	})

	t.Run("malformed json returns unknown", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not json`))
		}))
		defer srv.Close()
		apiBaseURL = srv.URL

		got := getCommitDate("owner/repo", "abc123")
		if got != "unknown" {
			t.Errorf("got %q, want %q", got, "unknown")
		}
	})
}

func TestGetRepoTagInfo(t *testing.T) {
	orig := apiBaseURL
	defer func() { apiBaseURL = orig }()

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
		apiBaseURL = srv.URL

		info, err := getRepoTagInfo("owner/repo")
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

	t.Run("no semver tags returns nil info", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"name":"latest","commit":{"sha":"aaa"}}]`))
		}))
		defer srv.Close()
		apiBaseURL = srv.URL

		info, err := getRepoTagInfo("owner/repo")
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
		apiBaseURL = srv.URL

		_, err := getRepoTagInfo("owner/repo")
		if err == nil {
			t.Fatal("expected error")
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
}

func TestFetchRepos(t *testing.T) {
	orig := apiBaseURL
	defer func() { apiBaseURL = orig }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/commits/") {
			w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-01T00:00:00Z"}}}`))
			return
		}
		w.Write([]byte(`[{"name":"v1.0.0","commit":{"sha":"abc1234"}}]`))
	}))
	defer srv.Close()
	apiBaseURL = srv.URL

	checked, errs := fetchRepos([]string{"owner/repo-a", "owner/repo-b"})

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
