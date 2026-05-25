package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const configFilePath = ".bumpflow.yaml"

func loadConfigFile() config {
	var cfg config
	f, err := os.Open(configFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", configFilePath, err)
		}
		return cfg
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
		val := strings.TrimSpace(parts[1])

		switch key {
		case "always_sha":
			cfg.useHash = val == "true"
		case "always_tag":
			cfg.useTag = val == "true"
		case "count":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.tagCount = n
			}
		case "update_all":
			cfg.updateAll = val == "true"
		case "verbose":
			cfg.verbose = val == "true"
		case "dry_run":
			cfg.dryRun = val == "true"
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error reading %s: %v\n", configFilePath, err)
	}
	return cfg
}
