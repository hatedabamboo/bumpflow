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
	}

	for _, tt := range tests {
		os.Args = tt.args
		got := parseArgs()
		if got != tt.want {
			t.Errorf("parseArgs(%v) = %+v, want %+v", tt.args[1:], got, tt.want)
		}
	}
}
