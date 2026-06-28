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
		rewriteFile(f, pattern, "v4", false)

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
		rewriteFile(f, pattern, "v5", false)

		data, _ := os.ReadFile(f)
		if string(data) != original {
			t.Errorf("file changed unexpectedly: got %q", string(data))
		}
	})

	t.Run("preserves inline comment after ref", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "test.yml")
		os.WriteFile(f, []byte("      uses: actions/checkout@v3 # pinned\n"), 0644)

		pattern := regexp.MustCompile(`(uses:\s+actions/checkout@)[^\s#]+`)
		rewriteFile(f, pattern, "v4", false)

		data, _ := os.ReadFile(f)
		want := "      uses: actions/checkout@v4 # pinned\n"
		if string(data) != want {
			t.Errorf("got %q, want %q", string(data), want)
		}
	})

	t.Run("does not panic on missing file", func(t *testing.T) {
		pattern := regexp.MustCompile(`(.*)`)
		rewriteFile("/nonexistent/path/file.yml", pattern, "x", false)
	})
}

func TestApplyUpdate(t *testing.T) {
	t.Run("writes tag without comment", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "ci.yml")
		os.WriteFile(f, []byte("      - uses: actions/checkout@v3\n"), 0644)

		a := action{actionRef: "actions/checkout", latestTag: "v4", files: []string{f}}
		applyUpdate(a, "v4", "", false)

		data, _ := os.ReadFile(f)
		if string(data) != "      - uses: actions/checkout@v4\n" {
			t.Errorf("got %q", string(data))
		}
	})

	t.Run("writes hash with tag comment", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "ci.yml")
		os.WriteFile(f, []byte("      - uses: actions/checkout@v3\n"), 0644)

		sha := "abc1234567890abc1234567890abc1234567890ab"
		a := action{actionRef: "actions/checkout", latestTag: "v4", files: []string{f}}
		applyUpdate(a, sha, "v4", false)

		data, _ := os.ReadFile(f)
		want := "      - uses: actions/checkout@" + sha + " # v4\n"
		if string(data) != want {
			t.Errorf("got %q, want %q", string(data), want)
		}
	})

	t.Run("replaces existing comment when updating hash", func(t *testing.T) {
		sha1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		sha2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		f := filepath.Join(t.TempDir(), "ci.yml")
		os.WriteFile(f, []byte("      - uses: actions/checkout@"+sha1+" # v3\n"), 0644)

		a := action{actionRef: "actions/checkout", files: []string{f}}
		applyUpdate(a, sha2, "v4", false)

		data, _ := os.ReadFile(f)
		want := "      - uses: actions/checkout@" + sha2 + " # v4\n"
		if string(data) != want {
			t.Errorf("got %q, want %q", string(data), want)
		}
	})
}

func TestApplyReplace(t *testing.T) {
	t.Run("tag to sha adds comment", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "ci.yml")
		os.WriteFile(f, []byte("      - uses: actions/checkout@v3\n"), 0644)

		sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		applyReplace("actions/checkout", "v3", sha, []string{f}, false)

		data, _ := os.ReadFile(f)
		want := "      - uses: actions/checkout@" + sha + " # v3\n"
		if string(data) != want {
			t.Errorf("got %q, want %q", string(data), want)
		}
	})

	t.Run("sha to tag removes comment", func(t *testing.T) {
		sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		f := filepath.Join(t.TempDir(), "ci.yml")
		os.WriteFile(f, []byte("      - uses: actions/checkout@"+sha+" # v3\n"), 0644)

		applyReplace("actions/checkout", sha, "v4", []string{f}, false)

		data, _ := os.ReadFile(f)
		if string(data) != "      - uses: actions/checkout@v4\n" {
			t.Errorf("got %q", string(data))
		}
	})

	t.Run("sha to tag without comment", func(t *testing.T) {
		sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		f := filepath.Join(t.TempDir(), "ci.yml")
		os.WriteFile(f, []byte("      - uses: actions/checkout@"+sha+"\n"), 0644)

		applyReplace("actions/checkout", sha, "v4", []string{f}, false)

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

	t.Run("select number then t returns tag without comment", func(t *testing.T) {
		ref, comment, ok := pickRef(a, config{}, reader("1\nt\n"))
		if !ok || ref != "v3.0.0" || comment != "" {
			t.Errorf("got (%q, %q, %v), want (v3.0.0, \"\", true)", ref, comment, ok)
		}
	})

	t.Run("select number then s returns sha with tag comment", func(t *testing.T) {
		ref, comment, ok := pickRef(a, config{}, reader("2\ns\n"))
		if !ok || ref != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || comment != "v2.0.0" {
			t.Errorf("got (%q, %q, %v), want (bbb..., v2.0.0, true)", ref, comment, ok)
		}
	})

	t.Run("empty input skips", func(t *testing.T) {
		ref, comment, ok := pickRef(a, config{}, reader("\n"))
		if ok || ref != "" || comment != "" {
			t.Errorf("got (%q, %q, %v), want (\"\", \"\", false)", ref, comment, ok)
		}
	})

	t.Run("non-integer input is invalid", func(t *testing.T) {
		ref, comment, ok := pickRef(a, config{}, reader("abc\n"))
		if ok || ref != "" || comment != "" {
			t.Errorf("got (%q, %q, %v), want (\"\", \"\", false)", ref, comment, ok)
		}
	})

	t.Run("out of range number is invalid", func(t *testing.T) {
		ref, comment, ok := pickRef(a, config{}, reader("5\n"))
		if ok || ref != "" || comment != "" {
			t.Errorf("got (%q, %q, %v), want (\"\", \"\", false)", ref, comment, ok)
		}
	})

	t.Run("unrecognized t/s choice skips", func(t *testing.T) {
		ref, comment, ok := pickRef(a, config{}, reader("1\nx\n"))
		if ok || ref != "" || comment != "" {
			t.Errorf("got (%q, %q, %v), want (\"\", \"\", false)", ref, comment, ok)
		}
	})

	t.Run("useTag skips t/s prompt and returns tag without comment", func(t *testing.T) {
		ref, comment, ok := pickRef(a, config{useTag: true}, reader("1\n"))
		if !ok || ref != "v3.0.0" || comment != "" {
			t.Errorf("got (%q, %q, %v), want (v3.0.0, \"\", true)", ref, comment, ok)
		}
	})

	t.Run("useHash skips t/s prompt and returns sha with tag comment", func(t *testing.T) {
		ref, comment, ok := pickRef(a, config{useHash: true}, reader("2\n"))
		if !ok || ref != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || comment != "v2.0.0" {
			t.Errorf("got (%q, %q, %v), want (bbb..., v2.0.0, true)", ref, comment, ok)
		}
	})
}

func TestFilterEntries(t *testing.T) {
	ci := "/fake/workflows/ci.yml"
	release := "/fake/workflows/release.yml"

	all := []rawEntry{
		{wfFile: ci, actionRef: "actions/checkout", ownerRepo: "actions/checkout"},
		{wfFile: ci, actionRef: "actions/setup-go", ownerRepo: "actions/setup-go"},
		{wfFile: release, actionRef: "actions/checkout", ownerRepo: "actions/checkout"},
	}

	tests := []struct {
		name         string
		targetFile   string
		targetAction string
		wantLen      int
	}{
		{"no filters returns all", "", "", 3},
		{"file filter matches ci.yml", ci, "", 2},
		{"file filter no match", "/fake/workflows/other.yml", "", 0},
		{"action filter matches checkout", "", "actions/checkout", 2},
		{"action filter no match", "", "actions/nonexistent", 0},
		{"both set entry matches both", ci, "actions/checkout", 1},
		{"file matches action does not", ci, "actions/nonexistent", 0},
		{"action matches file does not", "/fake/workflows/other.yml", "actions/checkout", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterEntries(all, tt.targetFile, tt.targetAction)
			if len(got) != tt.wantLen {
				t.Errorf("filterEntries returned %d entries, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestPrepareEntries(t *testing.T) {
	ci := "/fake/workflows/ci.yml"
	release := "/fake/workflows/release.yml"

	all := []rawEntry{
		{wfFile: ci, actionRef: "actions/checkout", ownerRepo: "actions/checkout"},
		{wfFile: ci, actionRef: "actions/setup-go", ownerRepo: "actions/setup-go"},
		{wfFile: release, actionRef: "actions/checkout", ownerRepo: "actions/checkout"},
	}

	t.Run("no filters returns all entries and sorted repos", func(t *testing.T) {
		entries, repos, err := prepareEntries(config{}, all)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 3 {
			t.Errorf("got %d entries, want 3", len(entries))
		}
		if len(repos) != 2 {
			t.Errorf("got %d repos, want 2", len(repos))
		}
		for i := 1; i < len(repos); i++ {
			if repos[i] < repos[i-1] {
				t.Errorf("repos not sorted: %v", repos)
			}
		}
	})

	t.Run("targetFile set entries found", func(t *testing.T) {
		entries, _, err := prepareEntries(config{targetFile: ci}, all)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("got %d entries, want 2", len(entries))
		}
	})

	t.Run("targetFile set no actions found", func(t *testing.T) {
		_, _, err := prepareEntries(config{targetFile: "/fake/workflows/other.yml"}, all)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "other.yml") {
			t.Errorf("error should mention file path, got: %v", err)
		}
	})

	t.Run("targetAction set action found", func(t *testing.T) {
		entries, _, err := prepareEntries(config{targetAction: "actions/checkout"}, all)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("got %d entries, want 2", len(entries))
		}
	})

	t.Run("targetAction set action not found", func(t *testing.T) {
		_, _, err := prepareEntries(config{targetAction: "actions/nonexistent"}, all)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "actions/nonexistent") {
			t.Errorf("error should mention action name, got: %v", err)
		}
	})

	t.Run("both set file not found", func(t *testing.T) {
		cfg := config{targetFile: "/fake/workflows/other.yml", targetAction: "actions/checkout"}
		_, _, err := prepareEntries(cfg, all)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "other.yml") {
			t.Errorf("error should mention file path, got: %v", err)
		}
	})

	t.Run("both set file found but action not in file", func(t *testing.T) {
		cfg := config{targetFile: ci, targetAction: "actions/nonexistent"}
		_, _, err := prepareEntries(cfg, all)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "actions/nonexistent") {
			t.Errorf("error should mention action name, got: %v", err)
		}
		if !strings.Contains(err.Error(), "ci.yml") {
			t.Errorf("error should mention the file, got: %v", err)
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
