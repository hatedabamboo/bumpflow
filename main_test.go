package main

import (
	"os"
	"testing"
)

func TestParseArgs(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	tests := []struct {
		args []string
		want config
	}{
		{
			[]string{"bumpflow"},
			config{tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "-t"},
			config{useTag: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "--tags"},
			config{useTag: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "-s"},
			config{useHash: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "--sha"},
			config{useHash: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "-A"},
			config{updateAll: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "--update-all"},
			config{updateAll: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "-r"},
			config{useReplace: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "--replace"},
			config{useReplace: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "-v"},
			config{verbose: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "-t", "-A"},
			config{useTag: true, updateAll: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "-n", "5"},
			config{tagCount: 5},
		},
		{
			[]string{"bumpflow", "--count", "3"},
			config{tagCount: 3},
		},
		{
			[]string{"bumpflow", "-d"},
			config{dryRun: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpflow", "--dry-run"},
			config{dryRun: true, tagCount: defaultTagCount},
		},
	}

	for _, tt := range tests {
		os.Args = tt.args
		got := parseArgs(config{})
		if got != tt.want {
			t.Errorf("parseArgs(%v) = %+v, want %+v", tt.args[1:], got, tt.want)
		}
	}
}

func TestParseArgsInheritsBase(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"bumpflow"}
	base := config{useHash: true, tagCount: 5, dryRun: true}
	got := parseArgs(base)

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
