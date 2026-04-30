package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

const defaultTagCount = 10

var version = "dev"

type config struct {
	useTag     bool
	tagCount   int
	useHash    bool
	updateAll  bool
	useReplace bool
	verbose    bool
}

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
  -t, --tags        Always use tags when updating (skip prompt)
  -n, --count       Number of latest tags to fetch (default 10)
  -s, --sha         Always use commit hashes when updating (skip prompt)
  -A, --update-all  Update all outdated actions without prompting
                    (defaults to hash; respects -t or -s if provided)
  -r, --replace     Convert pinned tags↔SHAs without upgrading versions
  -v, --verbose     Enable verbose logging
  -h, --help        Show this help
  -V, --version     Show version

Environment:
  GH_TOKEN  GitHub personal access token for authenticated API calls.
            Anonymous requests are limited to 60/hour.
  NO_COLOR  Disable colored output when set (any value).
`)
}

func parseArgs() config {
	var cfg config
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-t", "--tags":
			cfg.useTag = true
		case "-n", "--count":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: -n requires a value")
				os.Exit(1)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "Error: -n value must be a positive integer")
				os.Exit(1)
			}
			cfg.tagCount = n
		case "-s", "--sha":
			cfg.useHash = true
		case "-A", "--update-all":
			cfg.updateAll = true
		case "-r", "--replace":
			cfg.useReplace = true
		case "-v", "--verbose":
			cfg.verbose = true
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		case "-V", "--version":
			fmt.Println("bumpwf", version)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", arg)
			printUsage()
			os.Exit(1)
		}
	}
	if cfg.tagCount == 0 {
		cfg.tagCount = defaultTagCount
	}
	if cfg.useTag && cfg.useHash {
		fmt.Fprintln(os.Stderr, "Error: -t and -s are mutually exclusive.")
		os.Exit(1)
	}
	if cfg.updateAll && cfg.useReplace {
		fmt.Fprintln(os.Stderr, "Error: -A and -r are mutually exclusive.")
		os.Exit(1)
	}
	return cfg
}

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func main() {
	cfg := parseArgs()
	initLogger(cfg.verbose)

	if !isGitRepo() {
		fmt.Fprintln(os.Stderr, cRed("Error: not inside a git repository. Run from the repo root."))
		os.Exit(1)
	}

	if cfg.useReplace {
		replace()
		return
	}

	remaining, hadErrors := scan(cfg)

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
			if cfg.useTag {
				ref = a.latestTag
			}
			fmt.Printf("\nUpdating %s %s %s\n\n", bold(a.actionRef), cDim("→"), cGreen(ref))
			applyUpdate(a, ref)
		}
		fmt.Println(cGreen("\nAll actions updated!"))
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
		line, _ := reader.ReadString('\n')
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
		ref, ok := pickRef(a, cfg, reader)
		if !ok {
			fmt.Println(cYellow("  Skipped."))
			continue
		}
		fmt.Println()
		applyUpdate(a, ref)
		fmt.Println()
		fmt.Println(cGreen("  Done."))
	}

	if len(remaining) == 0 {
		fmt.Println(cGreen("\nAll actions updated!"))
	}
}
