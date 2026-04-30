package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestShortSHA(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc1234def5678", "abc1234"},
		{"abc1234", "abc1234"},
		{"abc123", "unknown"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		got := shortSHA(tt.input)
		if got != tt.want {
			t.Errorf("shortSHA(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRewriteFile(t *testing.T) {
	t.Run("updates matching line", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "test.yml")
		os.WriteFile(f, []byte("      uses: actions/checkout@v3\n"), 0644)

		pattern := regexp.MustCompile(`(uses:\s+actions/checkout@)[^\s#]+`)
		rewriteFile(f, pattern, "v4")

		data, _ := os.ReadFile(f)
		if string(data) != "      uses: actions/checkout@v4\n" {
			t.Errorf("got %q", string(data))
		}
	})

	t.Run("no-op when pattern does not match", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "test.yml")
		original := "      uses: actions/setup-go@v4\n"
		os.WriteFile(f, []byte(original), 0644)

		pattern := regexp.MustCompile(`(uses:\s+actions/checkout@)[^\s#]+`)
		rewriteFile(f, pattern, "v5")

		data, _ := os.ReadFile(f)
		if string(data) != original {
			t.Errorf("file changed unexpectedly: got %q", string(data))
		}
	})

	t.Run("preserves inline comment after ref", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "test.yml")
		os.WriteFile(f, []byte("      uses: actions/checkout@v3 # pinned\n"), 0644)

		pattern := regexp.MustCompile(`(uses:\s+actions/checkout@)[^\s#]+`)
		rewriteFile(f, pattern, "v4")

		data, _ := os.ReadFile(f)
		want := "      uses: actions/checkout@v4 # pinned\n"
		if string(data) != want {
			t.Errorf("got %q, want %q", string(data), want)
		}
	})

	t.Run("does not panic on missing file", func(t *testing.T) {
		pattern := regexp.MustCompile(`(.*)`)
		rewriteFile("/nonexistent/path/file.yml", pattern, "x")
	})
}

func TestApplyUpdate(t *testing.T) {
	f := filepath.Join(t.TempDir(), "ci.yml")
	os.WriteFile(f, []byte("      - uses: actions/checkout@v3\n"), 0644)

	a := action{
		actionRef: "actions/checkout",
		latestTag: "v4",
		files:     []string{f},
	}
	applyUpdate(a, "v4")

	data, _ := os.ReadFile(f)
	if string(data) != "      - uses: actions/checkout@v4\n" {
		t.Errorf("got %q", string(data))
	}
}

func TestApplyReplace(t *testing.T) {
	t.Run("tag to sha", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "ci.yml")
		os.WriteFile(f, []byte("      - uses: actions/checkout@v3\n"), 0644)

		applyReplace("actions/checkout", "v3", "abc1234567890abc1234567890abc1234567890ab", []string{f})

		data, _ := os.ReadFile(f)
		if string(data) != "      - uses: actions/checkout@abc1234567890abc1234567890abc1234567890ab\n" {
			t.Errorf("got %q", string(data))
		}
	})

	t.Run("sha to tag", func(t *testing.T) {
		sha := "abc1234567890abc1234567890abc1234567890ab"
		f := filepath.Join(t.TempDir(), "ci.yml")
		os.WriteFile(f, []byte("      - uses: actions/checkout@"+sha+"\n"), 0644)

		applyReplace("actions/checkout", sha, "v4", []string{f})

		data, _ := os.ReadFile(f)
		if string(data) != "      - uses: actions/checkout@v4\n" {
			t.Errorf("got %q", string(data))
		}
	})
}

func TestPickRef(t *testing.T) {
	a := action{
		availableTags: []tagInfo{
			{tag: "v3.0.0", sha: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			{tag: "v2.0.0", sha: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		},
	}

	reader := func(input string) *bufio.Reader {
		return bufio.NewReader(strings.NewReader(input))
	}

	t.Run("select number then t returns tag", func(t *testing.T) {
		ref, ok := pickRef(a, config{}, reader("1\nt\n"))
		if !ok || ref != "v3.0.0" {
			t.Errorf("got (%q, %v), want (v3.0.0, true)", ref, ok)
		}
	})

	t.Run("select number then s returns sha", func(t *testing.T) {
		ref, ok := pickRef(a, config{}, reader("2\ns\n"))
		if !ok || ref != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
			t.Errorf("got (%q, %v), want (bbb..., true)", ref, ok)
		}
	})

	t.Run("empty input skips", func(t *testing.T) {
		ref, ok := pickRef(a, config{}, reader("\n"))
		if ok || ref != "" {
			t.Errorf("got (%q, %v), want (\"\", false)", ref, ok)
		}
	})

	t.Run("non-integer input is invalid", func(t *testing.T) {
		ref, ok := pickRef(a, config{}, reader("abc\n"))
		if ok || ref != "" {
			t.Errorf("got (%q, %v), want (\"\", false)", ref, ok)
		}
	})

	t.Run("out of range number is invalid", func(t *testing.T) {
		ref, ok := pickRef(a, config{}, reader("5\n"))
		if ok || ref != "" {
			t.Errorf("got (%q, %v), want (\"\", false)", ref, ok)
		}
	})

	t.Run("unrecognized t/s choice skips", func(t *testing.T) {
		ref, ok := pickRef(a, config{}, reader("1\nx\n"))
		if ok || ref != "" {
			t.Errorf("got (%q, %v), want (\"\", false)", ref, ok)
		}
	})

	t.Run("useTag skips t/s prompt and returns tag", func(t *testing.T) {
		ref, ok := pickRef(a, config{useTag: true}, reader("1\n"))
		if !ok || ref != "v3.0.0" {
			t.Errorf("got (%q, %v), want (v3.0.0, true)", ref, ok)
		}
	})

	t.Run("useHash skips t/s prompt and returns sha", func(t *testing.T) {
		ref, ok := pickRef(a, config{useHash: true}, reader("2\n"))
		if !ok || ref != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
			t.Errorf("got (%q, %v), want (bbb..., true)", ref, ok)
		}
	})
}

func TestCollectEntries(t *testing.T) {
	dir := t.TempDir()
	wfDir := filepath.Join(dir, ".github", "workflows")
	os.MkdirAll(wfDir, 0755)

	content := `name: CI
jobs:
  build:
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - uses: actions/checkout@v4
      - uses: ./.local/action
`
	os.WriteFile(filepath.Join(wfDir, "ci.yml"), []byte(content), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	entries, ownerRepos := collectEntries()

	// 3 uses: lines match, but local action is skipped → 2 valid entries (checkout appears twice)
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}
	// only 2 unique owner/repos
	if len(ownerRepos) != 2 {
		t.Errorf("got %d ownerRepos, want 2", len(ownerRepos))
	}
	for _, e := range entries {
		if e.ownerRepo == "." || e.ownerRepo == "./.local" {
			t.Errorf("local action should be skipped, got ownerRepo %q", e.ownerRepo)
		}
	}
}
