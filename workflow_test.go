package main

import (
	"bufio"
	"errors"
	"fmt"
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

func replaceWith(s string) func([]string) string {
	return func(sub []string) string { return sub[1] + s }
}

func TestRewriteFile(t *testing.T) {
	t.Run("updates matching line", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "test.yml")
		os.WriteFile(f, []byte("      uses: actions/checkout@v3\n"), 0644)

		pattern := regexp.MustCompile(`(uses:\s+actions/checkout@)[^\s#]+`)
		rewriteFile(f, pattern, replaceWith("v4"), false)

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
		rewriteFile(f, pattern, replaceWith("v5"), false)

		data, _ := os.ReadFile(f)
		if string(data) != original {
			t.Errorf("file changed unexpectedly: got %q", string(data))
		}
	})

	t.Run("preserves inline comment after ref", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "test.yml")
		os.WriteFile(f, []byte("      uses: actions/checkout@v3 # pinned\n"), 0644)

		pattern := regexp.MustCompile(`(uses:\s+actions/checkout@)[^\s#]+`)
		rewriteFile(f, pattern, replaceWith("v4"), false)

		data, _ := os.ReadFile(f)
		want := "      uses: actions/checkout@v4 # pinned\n"
		if string(data) != want {
			t.Errorf("got %q, want %q", string(data), want)
		}
	})

	t.Run("does not panic on missing file", func(t *testing.T) {
		pattern := regexp.MustCompile(`(.*)`)
		rewriteFile("/nonexistent/path/file.yml", pattern, replaceWith("x"), false)
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

func TestApplyUpdateQuoted(t *testing.T) {
	sha := "abc1234567890abc1234567890abc1234567890ab"
	tests := []struct {
		name    string
		input   string
		ref     string
		comment string
		want    string
	}{
		{"double quotes to sha", `      - uses: "actions/cache@v3"`, sha, "v4.2.3", `      - uses: "actions/cache@` + sha + `" # v4.2.3`},
		{"single quotes to sha", `      - uses: 'actions/cache@v3'`, sha, "v4.2.3", `      - uses: 'actions/cache@` + sha + `' # v4.2.3`},
		{"double quotes to tag", `      - uses: "actions/cache@v3"`, "v4.2.3", "", `      - uses: "actions/cache@v4.2.3"`},
		{"quoted with old version comment", `      - uses: "actions/cache@` + sha + `" # v3`, sha, "v4.2.3", `      - uses: "actions/cache@` + sha + `" # v4.2.3`},
		{"unquoted still works", `      - uses: actions/cache@v3`, sha, "v4.2.3", `      - uses: actions/cache@` + sha + ` # v4.2.3`},
		{"mismatched quotes left alone", `      - uses: "actions/cache@v3'`, sha, "v4.2.3", `      - uses: "actions/cache@v3'`},
		{"unclosed quote left alone", `      - uses: "actions/cache@v3`, sha, "v4.2.3", `      - uses: "actions/cache@v3`},
		{"closing quote only left alone", `      - uses: actions/cache@v3"`, sha, "v4.2.3", `      - uses: actions/cache@v3"`},
		{"preserves CRLF", "      - uses: actions/cache@v3 # v3\r", "v4", "", "      - uses: actions/cache@v4\r"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "ci.yml")
			os.WriteFile(f, []byte(tt.input+"\n"), 0644)

			a := action{actionRef: "actions/cache", files: []string{f}}
			applyUpdate(a, tt.ref, tt.comment, false)

			data, _ := os.ReadFile(f)
			if string(data) != tt.want+"\n" {
				t.Errorf("got %q, want %q", string(data), tt.want+"\n")
			}
		})
	}
}

func TestApplyReplaceQuoted(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name  string
		input string
		from  string
		to    string
		want  string
	}{
		{"double quoted tag to sha", `      - uses: "actions/cache@v3"`, "v3", sha, `      - uses: "actions/cache@` + sha + `" # v3`},
		{"single quoted tag to sha", `      - uses: 'actions/cache@v3'`, "v3", sha, `      - uses: 'actions/cache@` + sha + `' # v3`},
		{"double quoted sha to tag", `      - uses: "actions/cache@` + sha + `" # v3`, sha, "v3", `      - uses: "actions/cache@v3"`},
		{"mismatched quotes left alone", `      - uses: 'actions/cache@v3"`, "v3", sha, `      - uses: 'actions/cache@v3"`},
		{"unclosed quote left alone", `      - uses: 'actions/cache@v3`, "v3", sha, `      - uses: 'actions/cache@v3`},
		{"other version not touched", `      - uses: actions/cache@v3.1`, "v3", sha, `      - uses: actions/cache@v3.1`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "ci.yml")
			os.WriteFile(f, []byte(tt.input+"\n"), 0644)

			applyReplace("actions/cache", tt.from, tt.to, []string{f}, false)

			data, _ := os.ReadFile(f)
			if string(data) != tt.want+"\n" {
				t.Errorf("got %q, want %q", string(data), tt.want+"\n")
			}
		})
	}
}

func TestKeptComment(t *testing.T) {
	tests := []struct {
		name     string
		comment  string
		wantLead string
		wantRest string
	}{
		{"none", "", "", ""},
		{"version with v", " # v1.2.3", " ", ""},
		{"version without v", " # 1.2", " ", ""},
		{"major only", "  #v6  ", " ", ""},
		{"version then note", " # v6.0.0 # keep: v6 breaks cgo", " ", "# keep: v6 breaks cgo"},
		{"note", "  # keep: v6 breaks cgo", "  ", "# keep: v6 breaks cgo"},
		{"version then text", " # v1.2.3 pinned", " ", "# v1.2.3 pinned"},
		{"empty comment", " #", " ", "#"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lead, rest := keptComment(tt.comment)
			if lead != tt.wantLead || rest != tt.wantRest {
				t.Errorf("keptComment(%q) = (%q, %q), want (%q, %q)", tt.comment, lead, rest, tt.wantLead, tt.wantRest)
			}
		})
	}
}

func TestApplyUpdateComments(t *testing.T) {
	sha := "abc1234567890abc1234567890abc1234567890ab"
	tests := []struct {
		name    string
		input   string
		ref     string
		comment string
		want    string
	}{
		{"tag keeps note", "      - uses: actions/setup-go@v5  # keep: v6 breaks cgo", "v6.0.0", "", "      - uses: actions/setup-go@v6.0.0  # keep: v6 breaks cgo"},
		{"sha adds version before note", "      - uses: actions/setup-go@v5  # keep: v6 breaks cgo", sha, "v6.0.0", "      - uses: actions/setup-go@" + sha + " # v6.0.0 # keep: v6 breaks cgo"},
		{"sha replaces version, keeps note", "      - uses: actions/setup-go@" + sha + " # v5.0.0 # keep: v6 breaks cgo", sha, "v6.0.0", "      - uses: actions/setup-go@" + sha + " # v6.0.0 # keep: v6 breaks cgo"},
		{"tag drops version, keeps note", "      - uses: actions/setup-go@" + sha + " # v5.0.0 # keep", "v6.0.0", "", "      - uses: actions/setup-go@v6.0.0 # keep"},
		{"version-only comment replaced", "      - uses: actions/setup-go@" + sha + " #  1.2 ", sha, "v6.0.0", "      - uses: actions/setup-go@" + sha + " # v6.0.0"},
		{"version-only comment dropped for tag", "      - uses: actions/setup-go@" + sha + " # v5", "v6.0.0", "", "      - uses: actions/setup-go@v6.0.0"},
		{"quoted keeps note", `      - uses: "actions/setup-go@v5" # keep`, sha, "v6.0.0", `      - uses: "actions/setup-go@` + sha + `" # v6.0.0 # keep`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "ci.yml")
			os.WriteFile(f, []byte(tt.input+"\n"), 0644)

			a := action{actionRef: "actions/setup-go", files: []string{f}}
			applyUpdate(a, tt.ref, tt.comment, false)

			data, _ := os.ReadFile(f)
			if string(data) != tt.want+"\n" {
				t.Errorf("got %q, want %q", string(data), tt.want+"\n")
			}
		})
	}
}

func TestApplyReplaceComments(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name  string
		input string
		from  string
		to    string
		want  string
	}{
		{"tag to sha keeps note", "      - uses: actions/setup-go@v5  # keep", "v5", sha, "      - uses: actions/setup-go@" + sha + " # v5 # keep"},
		{"sha to tag keeps note", "      - uses: actions/setup-go@" + sha + " # v5 # keep", sha, "v5", "      - uses: actions/setup-go@v5 # keep"},
		{"sha to tag drops version", "      - uses: actions/setup-go@" + sha + " # v5", sha, "v5", "      - uses: actions/setup-go@v5"},
		{"tag to sha replaces stale version", "      - uses: actions/setup-go@v5 # v4.9", "v5", sha, "      - uses: actions/setup-go@" + sha + " # v5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "ci.yml")
			os.WriteFile(f, []byte(tt.input+"\n"), 0644)

			applyReplace("actions/setup-go", tt.from, tt.to, []string{f}, false)

			data, _ := os.ReadFile(f)
			if string(data) != tt.want+"\n" {
				t.Errorf("got %q, want %q", string(data), tt.want+"\n")
			}
		})
	}
}

func TestActionRe(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantRef string
		wantTag string
	}{
		{"unquoted", "      - uses: actions/cache@v3", "actions/cache", "v3"},
		{"unquoted with comment", "      - uses: actions/cache@v3 # pinned", "actions/cache", "v3"},
		{"double quotes", `      - uses: "actions/cache@v3"`, "actions/cache", "v3"},
		{"single quotes", `      - uses: 'actions/cache@v3'`, "actions/cache", "v3"},
		{"quoted with comment", `      - uses: "actions/cache@v3" # v3`, "actions/cache", "v3"},
		{"no dash", `        uses: "github/codeql-action/init@v3"`, "github/codeql-action/init", "v3"},
		{"mismatched quotes", `      - uses: "actions/cache@v3'`, "", ""},
		{"unclosed quote", `      - uses: "actions/cache@v3`, "", ""},
		{"closing quote only", `      - uses: actions/cache@v3"`, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotRef, gotTag string
			if m := actionRe.FindStringSubmatch(tt.line); m != nil && m[1] == m[4] {
				gotRef, gotTag = m[2], m[3]
			}
			if gotRef != tt.wantRef || gotTag != tt.wantTag {
				t.Errorf("got (%q, %q), want (%q, %q)", gotRef, gotTag, tt.wantRef, tt.wantTag)
			}
		})
	}
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
      - uses: "actions/cache@v3"
      - uses: 'actions/cache@v4'
      - uses: "actions/cache@v5'
`
	os.WriteFile(filepath.Join(wfDir, "ci.yml"), []byte(content), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	entries, ownerRepos := collectEntries()

	if len(entries) != 5 {
		t.Errorf("got %d entries, want 5", len(entries))
	}
	if len(ownerRepos) != 3 {
		t.Errorf("got %d ownerRepos, want 3", len(ownerRepos))
	}
	for _, e := range entries {
		if e.ownerRepo == "." || e.ownerRepo == "./.local" {
			t.Errorf("local action should be skipped, got ownerRepo %q", e.ownerRepo)
		}
	}
}

func TestOutdatedActions(t *testing.T) {
	latestSHA := strings.Repeat("a", 40)
	oldSHA := strings.Repeat("b", 40)
	unknownSHA := strings.Repeat("c", 40)
	checked := map[string]*repoInfo{
		"actions/setup-go": {
			latest: tagInfo{tag: "v6.0.2", sha: latestSHA},
			tags: map[string]string{
				"v6":     latestSHA,
				"v6.0":   latestSHA,
				"v6.0.2": latestSHA,
				"v6.0.1": oldSHA,
				"v5":     oldSHA,
				"v5.5.0": oldSHA,
			},
		},
	}

	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{"floating major at latest commit", "v6", false},
		{"floating minor at latest commit", "v6.0", false},
		{"exact latest", "v6.0.2", false},
		{"sha of latest", latestSHA, false},
		{"older major", "v5", true},
		{"older patch", "v6.0.1", true},
		{"sha of older tag", oldSHA, true},
		{"sha without tag", unknownSHA, false},
		{"tag not in list but older", "v4", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []rawEntry{{actionRef: "actions/setup-go", ownerRepo: "actions/setup-go", currentTag: tt.tag, wfFile: "ci.yml"}}
			got := outdatedActions(entries, checked)
			if (len(got) == 1) != tt.want {
				t.Errorf("outdated = %v, want %v (got %+v)", len(got) == 1, tt.want, got)
			}
		})
	}

	t.Run("groups versions and files per action", func(t *testing.T) {
		entries := []rawEntry{
			{actionRef: "actions/setup-go", ownerRepo: "actions/setup-go", currentTag: "v5", wfFile: "b.yml"},
			{actionRef: "actions/setup-go", ownerRepo: "actions/setup-go", currentTag: "v6", wfFile: "c.yml"},
			{actionRef: "actions/setup-go", ownerRepo: "actions/setup-go", currentTag: "v6.0.1", wfFile: "a.yml"},
			{actionRef: "actions/setup-go", ownerRepo: "actions/setup-go", currentTag: "v5", wfFile: "a.yml"},
			{actionRef: "other/repo", ownerRepo: "other/repo", currentTag: "v1", wfFile: "a.yml"},
		}
		got := outdatedActions(entries, checked)
		if len(got) != 1 {
			t.Fatalf("got %d actions, want 1", len(got))
		}
		a := got[0]
		if len(a.currentVersions) != 2 {
			t.Errorf("currentVersions = %v, want v5 and v6.0.1", a.currentVersions)
		}
		if strings.Join(a.files, ",") != "a.yml,b.yml" {
			t.Errorf("files = %v, want [a.yml b.yml]", a.files)
		}
		if a.latestTag != "v6.0.2" || a.latestSHA != latestSHA {
			t.Errorf("latest = %s %s", a.latestTag, a.latestSHA)
		}
	})
}

func TestReplaceItems(t *testing.T) {
	latestSHA := strings.Repeat("a", 40)
	checked := map[string]*repoInfo{
		"actions/setup-go": {
			latest: tagInfo{tag: "v6.0.2", sha: latestSHA},
			tags:   map[string]string{"v6": latestSHA, "v6.0.2": latestSHA},
		},
	}
	tests := []struct {
		name       string
		tag        string
		wantTarget string
	}{
		{"floating tag to sha", "v6", latestSHA},
		{"exact tag to sha", "v6.0.2", latestSHA},
		{"sha to best tag", latestSHA, "v6.0.2"},
		{"unknown tag skipped", "v1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []rawEntry{{actionRef: "actions/setup-go", ownerRepo: "actions/setup-go", currentTag: tt.tag, wfFile: "ci.yml"}}
			items := replaceItems(entries, checked)
			var got string
			if len(items) == 1 {
				got = items[0].target
			}
			if got != tt.wantTarget {
				t.Errorf("target = %q, want %q", got, tt.wantTarget)
			}
		})
	}
}

func TestFormatFetchErrors(t *testing.T) {
	rateLimit := errors.New("GitHub API rate limit exceeded — resets at 15:04 UTC")
	notFound := errors.New("repository not found — check if it's private or the action name is correct (HTTP 404)")
	wrap := func(repo string, err error) error { return fmt.Errorf("fetching tags for %s: %w", repo, err) }

	tests := []struct {
		name string
		errs map[string]error
		want string
	}{
		{
			name: "single error",
			errs: map[string]error{"actions/checkout": wrap("actions/checkout", rateLimit)},
			want: "\nFailed to fetch 1 repo(s):\n" +
				"  GitHub API rate limit exceeded — resets at 15:04 UTC\n" +
				"    actions/checkout\n",
		},
		{
			name: "same error grouped, distinct errors listed once",
			errs: map[string]error{
				"actions/setup-go": wrap("actions/setup-go", rateLimit),
				"actions/checkout": wrap("actions/checkout", rateLimit),
				"myorg/private":    wrap("myorg/private", notFound),
			},
			want: "\nFailed to fetch 3 repo(s):\n" +
				"  GitHub API rate limit exceeded — resets at 15:04 UTC\n" +
				"    actions/checkout, actions/setup-go\n" +
				"  repository not found — check if it's private or the action name is correct (HTTP 404)\n" +
				"    myorg/private\n",
		},
		{
			name: "unwrapped error used as is",
			errs: map[string]error{"a/b": errors.New("HTTP 500")},
			want: "\nFailed to fetch 1 repo(s):\n  HTTP 500\n    a/b\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatFetchErrors(tt.errs); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
