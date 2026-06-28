package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type config struct {
	dryRun       bool
	targetAction string
	targetFile   string
	tagCount     int
	updateAll    bool
	useHash      bool
	useReplace   bool
	useTag       bool
	verbose      bool
}

func loadConfigFile(path string) (config, bool) {
	var cfg config
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", path, err)
		}
		return cfg, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		raw := strings.TrimSpace(parts[1])

		var val string
		if len(raw) >= 2 && ((raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'')) {
			// Quoted value: strip quotes only; preserve any '#' inside as literal content.
			val = raw[1 : len(raw)-1]
		} else {
			// Unquoted value: strip inline comment before using.
			if i := strings.Index(raw, "#"); i >= 0 {
				raw = strings.TrimSpace(raw[:i])
			}
			val = raw
		}

		switch key {
		case "always_sha":
			cfg.useHash = val == "true"
		case "always_tag":
			cfg.useTag = val == "true"
		case "count":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.tagCount = n
			}
		case "dry_run":
			cfg.dryRun = val == "true"
		case "target_action":
			cfg.targetAction = val
		case "target_file":
			cfg.targetFile = val
		case "update_all":
			cfg.updateAll = val == "true"
		case "verbose":
			cfg.verbose = val == "true"
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error reading %s: %v\n", path, err)
	}
	return cfg, true
}
