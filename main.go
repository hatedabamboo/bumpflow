package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

const defaultTagCount = 10
const configFilePath = ".bumpflow.yaml"

var version = "dev"

func initLogger(v bool) {
	level := slog.LevelWarn
	if v {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

func printUsage() {
	fmt.Printf("Usage: %s [options]\n", os.Args[0])
	fmt.Print(`
Options:
  -a, --action      Update only the specified action (owner/repo)
  -A, --update-all  Update all outdated actions without prompting
                    (defaults to hash; respects -t or -s if provided)
  -d, --dry-run     Preview changes without modifying any files
  -h, --help        Show this help
  -f, --file        Update only actions in the specified workflow file
  -n, --count       Number of latest tags to fetch (default 10)
  -r, --replace     Convert pinned tags↔SHAs without upgrading versions
  -s, --sha         Always use commit hashes when updating (skip prompt)
  -t, --tags        Always use tags when updating (skip prompt)
  -v, --verbose     Enable verbose logging
  -V, --version     Show version

Environment:
  GH_TOKEN  GitHub personal access token for authenticated API calls.
            Anonymous requests are limited to 60/hour.
  NO_COLOR  Disable colored output when set (any value).

Config file (.bumpflow.yaml):
  Place a .bumpflow.yaml at the repo root to set persistent defaults.
  CLI flags always override config file settings.

  always_sha: true    # same as -s
  always_tag: false   # same as -t
  count: 5            # same as -n
  dry_run: false      # same as -d
  target_action: ""   # same as -a
  target_file: ""     # same as -f
  update_all: false   # same as -A
  verbose: false      # same as -v
`)
}

func parseArgs(base config) (config, error) {
	cfg := base
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-a", "--action":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("Error: -a requires a value")
			}
			i++
			val := args[i]
			if !strings.Contains(val, "/") {
				return cfg, fmt.Errorf("Error: action must be in owner/repo format")
			}
			cfg.targetAction = val
		case "-A", "--update-all":
			cfg.updateAll = true
		case "-d", "--dry-run":
			cfg.dryRun = true
		case "-f", "--file":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("Error: -f requires a value")
			}
			i++
			cfg.targetFile = args[i]
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		case "-n", "--count":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("Error: -n requires a value")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return cfg, fmt.Errorf("Error: -n value must be a positive integer")
			}
			cfg.tagCount = n
		case "-r", "--replace":
			cfg.useReplace = true
		case "-s", "--sha":
			cfg.useHash = true
		case "-t", "--tags":
			cfg.useTag = true
		case "-v", "--verbose":
			cfg.verbose = true
		case "-V", "--version":
			fmt.Println("bumpflow", version)
			os.Exit(0)
		default:
			return cfg, fmt.Errorf("Unknown flag: %s", arg)
		}
	}
	if cfg.tagCount == 0 {
		cfg.tagCount = defaultTagCount
	}
	if cfg.useTag && cfg.useHash {
		return cfg, fmt.Errorf("Error: -t and -s are mutually exclusive.")
	}
	if cfg.updateAll && cfg.useReplace {
		return cfg, fmt.Errorf("Error: -A and -r are mutually exclusive.")
	}
	return cfg, nil
}

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func validateTargetFile(file string) error {
	if file == "" {
		return nil
	}
	if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("Error: file %s does not exist", file)
	} else if err != nil {
		return fmt.Errorf("Error: cannot access file %s: %w", file, err)
	}
	return nil
}

func main() {
	fileCfg, cfgFileFound := loadConfigFile(configFilePath)
	cfg, err := parseArgs(fileCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, cRed(err.Error()))
		os.Exit(1)
	}
	if err := validateTargetFile(cfg.targetFile); err != nil {
		fmt.Fprintln(os.Stderr, cRed(err.Error()))
		os.Exit(1)
	}
	initLogger(cfg.verbose)
	if cfgFileFound {
		slog.Debug("config file loaded", "path", configFilePath,
			"always_sha", fileCfg.useHash,
			"always_tag", fileCfg.useTag,
			"count", fileCfg.tagCount,
			"dry_run", fileCfg.dryRun,
			"target_action", fileCfg.targetAction,
			"target_file", fileCfg.targetFile,
			"update_all", fileCfg.updateAll,
			"verbose", fileCfg.verbose,
		)
	} else {
		slog.Debug("no config file found", "path", configFilePath)
	}

	if !isGitRepo() {
		fmt.Fprintln(os.Stderr, cRed("Error: not inside a git repository. Run from the repo root."))
		os.Exit(1)
	}

	if cfg.dryRun {
		fmt.Println(cYellow("Dry run mode — no files will be modified.\n"))
	}

	if cfg.useReplace {
		if err := replace(cfg); err != nil {
			fmt.Fprintln(os.Stderr, cRed("Error: "+err.Error()))
			os.Exit(1)
		}
		return
	}

	remaining, hadErrors, err := scan(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, cRed("Error: "+err.Error()))
		os.Exit(1)
	}

	if len(remaining) == 0 {
		if !hadErrors {
			fmt.Println(cGreen("\nAll actions are up to date!"))
		}
		return
	}

	if cfg.updateAll {
		fmt.Printf("\n%s\n", bold(fmt.Sprintf("Updating all %d outdated action(s)...", len(remaining))))
		for _, a := range remaining {
			ref := a.latestSHA
			comment := a.latestTag
			if cfg.useTag {
				ref = a.latestTag
				comment = ""
			}
			fmt.Printf("\nUpdating %s %s %s\n\n", bold(a.actionRef), cDim("→"), cGreen(ref))
			applyUpdate(a, ref, comment, cfg.dryRun)
		}
		if cfg.dryRun {
			fmt.Println(cYellow("\nDry run complete — no files were modified."))
		} else {
			fmt.Println(cGreen("\nAll actions updated!"))
		}
		return
	}

	reader := bufio.NewReader(os.Stdin)

	for len(remaining) > 0 {
		fmt.Printf("\n%s\n\n", bold(fmt.Sprintf("Outdated action(s) remaining: %d", len(remaining))))
		for i, a := range remaining {
			tags := make([]string, 0, len(a.currentVersions))
			for t := range a.currentVersions {
				if hashRe.MatchString(t) {
					t = shortSHA(t)
				}
				tags = append(tags, t)
			}
			sort.Strings(tags)
			fmt.Printf("  [%d] %s: %s %s %s %s  %s\n",
				i+1, bold(a.actionRef),
				cYellow(strings.Join(tags, ", ")),
				cDim("→"),
				cGreen(a.latestTag),
				cDim("("+shortSHA(a.latestSHA)+")"),
				cDim("committed on "+a.latestDate),
			)
		}

		fmt.Println()
		fmt.Print("Which action to update? (number, or q to quit): ")
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)

		if strings.ToLower(line) == "q" {
			break
		}

		choice, err := strconv.Atoi(line)
		if err != nil || choice < 1 || choice > len(remaining) {
			fmt.Println(cRed("Invalid choice."))
			continue
		}

		a := remaining[choice-1]
		remaining = append(remaining[:choice-1], remaining[choice:]...)

		fmt.Printf("\nUpdating %s:\n", bold(a.actionRef))
		ref, comment, ok := pickRef(a, cfg, reader)
		if !ok {
			fmt.Println(cYellow("  Skipped."))
			continue
		}
		fmt.Println()
		applyUpdate(a, ref, comment, cfg.dryRun)
		fmt.Println()
		fmt.Println(cGreen("  Done."))
	}

	if len(remaining) == 0 {
		if cfg.dryRun {
			fmt.Println(cYellow("\nDry run complete — no files were modified."))
		} else {
			fmt.Println(cGreen("\nAll actions updated!"))
		}
	}
}
