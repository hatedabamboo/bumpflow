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
			[]string{"bumpwf"},
			config{tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "-t"},
			config{useTag: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "--tags"},
			config{useTag: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "-s"},
			config{useHash: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "--sha"},
			config{useHash: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "-A"},
			config{updateAll: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "--update-all"},
			config{updateAll: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "-r"},
			config{useReplace: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "--replace"},
			config{useReplace: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "-v"},
			config{verbose: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "-t", "-A"},
			config{useTag: true, updateAll: true, tagCount: defaultTagCount},
		},
		{
			[]string{"bumpwf", "-n", "5"},
			config{tagCount: 5},
		},
		{
			[]string{"bumpwf", "--count", "3"},
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
