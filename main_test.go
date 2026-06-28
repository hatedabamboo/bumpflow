package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	tests := []struct {
		args    []string
		want    config
		wantErr error
	}{
		{
			[]string{"bumpflow"},
			config{tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "-t"},
			config{useTag: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "--tags"},
			config{useTag: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "-s"},
			config{useHash: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "--sha"},
			config{useHash: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "-A"},
			config{updateAll: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "--update-all"},
			config{updateAll: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "-r"},
			config{useReplace: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "--replace"},
			config{useReplace: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "-v"},
			config{verbose: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "-t", "-A"},
			config{useTag: true, updateAll: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "-n", "5"},
			config{tagCount: 5},
			nil,
		},
		{
			[]string{"bumpflow", "--count", "3"},
			config{tagCount: 3},
			nil,
		},
		{
			[]string{"bumpflow", "-d"},
			config{dryRun: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "--dry-run"},
			config{dryRun: true, tagCount: defaultTagCount},
			nil,
		},
		{
			[]string{"bumpflow", "-f", ".github/workflows/ci.yml"},
			config{tagCount: defaultTagCount, targetFile: ".github/workflows/ci.yml"},
			nil,
		},
		{
			[]string{"bumpflow", "--file", ".github/workflows/ci.yml"},
			config{tagCount: defaultTagCount, targetFile: ".github/workflows/ci.yml"},
			nil,
		},
		{
			[]string{"bumpflow", "-a", "actions/checkout"},
			config{tagCount: defaultTagCount, targetAction: "actions/checkout"},
			nil,
		},
		{
			[]string{"bumpflow", "--action", "actions/checkout"},
			config{tagCount: defaultTagCount, targetAction: "actions/checkout"},
			nil,
		},
		{
			[]string{"bumpflow", "-f", ".github/workflows/ci.yml", "-a", "actions/checkout"},
			config{tagCount: defaultTagCount, targetFile: ".github/workflows/ci.yml", targetAction: "actions/checkout"},
			nil,
		},
		{
			[]string{"bumpflow", "-f", ".github/workflows/ci.yml", "-A"},
			config{tagCount: defaultTagCount, updateAll: true, targetFile: ".github/workflows/ci.yml"},
			nil,
		},
		{
			[]string{"bumpflow", "-a", "invalidvalue"},
			config{},
			fmt.Errorf("Error: action must be in owner/repo format"),
		},
		{
			[]string{"bumpflow", "-a"},
			config{},
			fmt.Errorf("Error: -a requires a value"),
		},
		{
			[]string{"bumpflow", "-f"},
			config{},
			fmt.Errorf("Error: -f requires a value"),
		},
		{
			[]string{"bumpflow", "-n", "abc"},
			config{},
			fmt.Errorf("Error: -n value must be a positive integer"),
		},
		{
			[]string{"bumpflow", "-t", "-s"},
			config{},
			fmt.Errorf("Error: -t and -s are mutually exclusive."),
		},
		{
			[]string{"bumpflow", "-A", "-r"},
			config{},
			fmt.Errorf("Error: -A and -r are mutually exclusive."),
		},
	}

	for _, tt := range tests {
		os.Args = tt.args
		got, err := parseArgs(config{})
		if tt.wantErr != nil {
			if err == nil {
				t.Errorf("parseArgs(%v) = nil error, want %v", tt.args[1:], tt.wantErr)
				continue
			}
			if err.Error() != tt.wantErr.Error() {
				t.Errorf("parseArgs(%v) error = %v, want %v", tt.args[1:], err, tt.wantErr)
			}
		} else if err != nil {
			t.Errorf("parseArgs(%v) unexpected error: %v", tt.args[1:], err)
		} else if got != tt.want {
			t.Errorf("parseArgs(%v) = %+v, want %+v", tt.args[1:], got, tt.want)
		}
	}
}

func TestFileExistsValidation(t *testing.T) {
	// Create a temp file for the positive test
	tmpFile, err := os.CreateTemp("", "test-workflow.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	t.Run("accepts existing file", func(t *testing.T) {
		err := validateTargetFile(tmpFile.Name())
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("accepts empty file", func(t *testing.T) {
		err := validateTargetFile("")
		if err != nil {
			t.Errorf("expected no error for empty file, got: %v", err)
		}
	})

	t.Run("rejects non-existent file", func(t *testing.T) {
		err := validateTargetFile("/nonexistent/path/workflow.yml")
		if err == nil {
			t.Fatal("expected error for non-existent file")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("error should mention 'does not exist', got: %q", err.Error())
		}
		if !strings.Contains(err.Error(), "/nonexistent/path/workflow.yml") {
			t.Errorf("error should contain the file path, got: %q", err.Error())
		}
	})
}

func TestParseArgsActionFormat(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	t.Run("accepts valid owner/repo", func(t *testing.T) {
		os.Args = []string{"bumpflow", "-a", "actions/checkout"}
		got, err := parseArgs(config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.targetAction != "actions/checkout" {
			t.Errorf("got %q, want actions/checkout", got.targetAction)
		}
	})

	t.Run("rejects value without slash", func(t *testing.T) {
		os.Args = []string{"bumpflow", "-a", "invalidvalue"}
		_, err := parseArgs(config{})
		if err == nil {
			t.Fatal("expected error for invalid action format")
		}
		if err.Error() != "Error: action must be in owner/repo format" {
			t.Errorf("got %q, want %q", err.Error(), "Error: action must be in owner/repo format")
		}
	})
}

func TestParseArgsInheritsBase(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"bumpflow"}
	base := config{useHash: true, tagCount: 5, dryRun: true}
	got, err := parseArgs(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !got.useHash {
		t.Error("expected useHash inherited from base")
	}
	if got.tagCount != 5 {
		t.Errorf("expected tagCount=5 from base, got %d", got.tagCount)
	}
	if !got.dryRun {
		t.Error("expected dryRun inherited from base")
	}
}
