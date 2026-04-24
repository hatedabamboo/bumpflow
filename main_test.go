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
			config{},
		},
		{
			[]string{"bumpwf", "-t"},
			config{useTag: true},
		},
		{
			[]string{"bumpwf", "--tags"},
			config{useTag: true},
		},
		{
			[]string{"bumpwf", "-s"},
			config{useHash: true},
		},
		{
			[]string{"bumpwf", "--sha"},
			config{useHash: true},
		},
		{
			[]string{"bumpwf", "-A"},
			config{updateAll: true},
		},
		{
			[]string{"bumpwf", "--update-all"},
			config{updateAll: true},
		},
		{
			[]string{"bumpwf", "-r"},
			config{useReplace: true},
		},
		{
			[]string{"bumpwf", "--replace"},
			config{useReplace: true},
		},
		{
			[]string{"bumpwf", "-v"},
			config{verbose: true},
		},
		{
			[]string{"bumpwf", "-t", "-A"},
			config{useTag: true, updateAll: true},
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
