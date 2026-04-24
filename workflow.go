package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const workflowsDir = ".github/workflows"

var (
	actionRe = regexp.MustCompile(`^\s+(?:-\s+)?uses:\s+([a-zA-Z0-9_./-]+/[a-zA-Z0-9_./%-]+)@(\S+)`)
	hashRe   = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type rawEntry struct {
	actionRef  string
	currentTag string
	ownerRepo  string
	wfFile     string
}

type action struct {
	actionRef       string
	ownerRepo       string
	currentVersions map[string]struct{}
	latestTag       string
	latestSHA       string
	latestDate      string
	files           []string
}

type replaceItem struct {
	actionRef string
	current   string
	target    string
	files     []string
}

func shortSHA(sha string) string {
	if len(sha) >= 7 {
		return sha[:7]
	}
	return "unknown"
}

// rewriteFile applies pattern to wfFile, replacing the capture group with replacement.
// Padding must be applied to raw strings before colorizing to keep column widths correct.
func rewriteFile(wfFile string, pattern *regexp.Regexp, replacement string) {
	slog.Debug("reading workflow file", "file", wfFile)
	data, err := os.ReadFile(wfFile)
	if err != nil {
		fmt.Printf("  %s %s: %v\n", cRed("Error reading"), wfFile, err)
		return
	}
	updated := pattern.ReplaceAll(data, []byte("${1}"+replacement))
	if string(updated) == string(data) {
		return
	}
	slog.Debug("writing workflow file", "file", wfFile)
	if err := os.WriteFile(wfFile, updated, 0644); err != nil {
		fmt.Printf("  %s %s: %v\n", cRed("Error writing"), wfFile, err)
		return
	}
	fmt.Printf("  %s %s\n", cGreen("Updated"), wfFile)
}

func applyUpdate(a action, ref string) {
	pattern := regexp.MustCompile(`(uses:\s+` + regexp.QuoteMeta(a.actionRef) + `@)[^\s#]+`)
	for _, wfFile := range a.files {
		rewriteFile(wfFile, pattern, ref)
	}
}

func applyReplace(actionRef, from, to string, files []string) {
	pattern := regexp.MustCompile(`(uses:\s+` + regexp.QuoteMeta(actionRef) + `@)` + regexp.QuoteMeta(from))
	for _, wfFile := range files {
		rewriteFile(wfFile, pattern, to)
	}
}

func collectEntries() ([]rawEntry, []string) {
	var wfFiles []string
	for _, pat := range []string{"*.yml", "*.yaml"} {
		matches, _ := filepath.Glob(filepath.Join(workflowsDir, pat))
		wfFiles = append(wfFiles, matches...)
	}
	sort.Strings(wfFiles)

	seen := map[string]struct{}{}
	var entries []rawEntry
	var ownerRepos []string

	for _, wfFile := range wfFiles {
		slog.Debug("reading workflow file", "file", wfFile)
		data, err := os.ReadFile(wfFile)
		if err != nil {
			slog.Warn("could not read workflow file", "file", wfFile, "error", err)
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			m := actionRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			actionRef, currentTag := m[1], m[2]
			if strings.HasPrefix(actionRef, ".") {
				continue
			}
			ownerRepo := strings.Join(strings.SplitN(actionRef, "/", 3)[:2], "/")
			entries = append(entries, rawEntry{actionRef, currentTag, ownerRepo, wfFile})
			if _, ok := seen[ownerRepo]; !ok {
				seen[ownerRepo] = struct{}{}
				ownerRepos = append(ownerRepos, ownerRepo)
			}
		}
	}
	return entries, ownerRepos
}

func printFetchErrors(fetchErrs map[string]error) {
	if len(fetchErrs) == 0 {
		return
	}
	if ghToken == "" {
		fmt.Println(cRed("\nEncountered an error: most likely hit the GitHub API rate limit. Anonymous GitHub API requests are limited to 60/hour — set GH_TOKEN environment variable or use a VPN."))
	} else {
		fmt.Println(cRed("\nEncountered an error fetching some repositories. Check your GH_TOKEN and network connectivity."))
	}
}

func scan() ([]action, bool) {
	entries, ownerRepos := collectEntries()

	fmt.Printf("Fetching %d repo(s)...\n", len(ownerRepos))
	checked, fetchErrs := fetchRepos(ownerRepos)

	installedVersions := map[string][]string{}
	seenInstalled := map[string]map[string]struct{}{}
	for _, e := range entries {
		if seenInstalled[e.ownerRepo] == nil {
			seenInstalled[e.ownerRepo] = map[string]struct{}{}
		}
		if _, ok := seenInstalled[e.ownerRepo][e.currentTag]; !ok {
			seenInstalled[e.ownerRepo][e.currentTag] = struct{}{}
			display := e.currentTag
			if hashRe.MatchString(e.currentTag) {
				display = shortSHA(e.currentTag)
			}
			installedVersions[e.ownerRepo] = append(installedVersions[e.ownerRepo], display)
		}
	}

	sortedRepos := append([]string(nil), ownerRepos...)
	sort.Strings(sortedRepos)
	fmt.Println()
	fmt.Printf("  %s %s %s\n",
		bold(fmt.Sprintf("%-45s", "Action")),
		bold(fmt.Sprintf("%-30s", "Installed version")),
		bold("Latest version"),
	)
	fmt.Printf("  %s %s %s\n",
		cDim(fmt.Sprintf("%-45s", "------")),
		cDim(fmt.Sprintf("%-30s", "-----------------")),
		cDim("--------------"),
	)
	for _, repo := range sortedRepos {
		info := checked[repo]
		installed := strings.Join(installedVersions[repo], ", ")
		col1 := fmt.Sprintf("%-45s", repo)
		col2 := fmt.Sprintf("%-30s", installed)
		if info != nil {
			fmt.Printf("  %s %s %s %s\n",
				col1,
				cYellow(col2),
				cGreen(info.latest.tag),
				cDim("("+shortSHA(info.latest.sha)+")"),
			)
		} else if _, ok := fetchErrs[repo]; ok {
			fmt.Printf("  %s %s %s\n", col1, col2, cRed("Error"))
		} else {
			fmt.Printf("  %s %s %s\n", col1, col2, cYellow("not found"))
		}
	}

	printFetchErrors(fetchErrs)

	actions := map[string]*action{}
	for _, e := range entries {
		info := checked[e.ownerRepo]
		if info == nil {
			continue
		}
		effectiveTag := e.currentTag
		if hashRe.MatchString(e.currentTag) {
			effectiveTag = bestTagForSHA(info, e.currentTag)
			if effectiveTag == "" {
				continue
			}
		}
		if !versionGreater(info.latest.tag, effectiveTag) {
			continue
		}
		if _, exists := actions[e.actionRef]; !exists {
			actions[e.actionRef] = &action{
				actionRef:       e.actionRef,
				ownerRepo:       e.ownerRepo,
				currentVersions: map[string]struct{}{},
				latestTag:       info.latest.tag,
				latestSHA:       info.latest.sha,
				latestDate:      info.latest.date,
			}
		}
		actions[e.actionRef].currentVersions[e.currentTag] = struct{}{}

		if !slices.Contains(actions[e.actionRef].files, e.wfFile) {
			actions[e.actionRef].files = append(actions[e.actionRef].files, e.wfFile)
		}
	}

	keys := make([]string, 0, len(actions))
	for k := range actions {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]action, 0, len(keys))
	for _, k := range keys {
		a := *actions[k]
		sort.Strings(a.files)
		result = append(result, a)
	}
	return result, len(fetchErrs) > 0
}

func pickRef(a action, cfg config, r *bufio.Reader) (string, bool) {
	if cfg.useTag {
		return a.latestTag, true
	}
	if cfg.useHash {
		return a.latestSHA, true
	}
	fmt.Printf("  [t] Tag:  %s  %s\n", cGreen(a.latestTag), cDim("(committed on "+a.latestDate+")"))
	fmt.Printf("  [s] SHA:  %s  %s\n", cCyan(a.latestSHA), cDim("(committed on "+a.latestDate+")"))
	fmt.Println("  [Enter] Skip")
	fmt.Print("  Use tag or hash? (t/s/Enter): ")
	line, _ := r.ReadString('\n')
	switch strings.TrimSpace(line) {
	case "t":
		return a.latestTag, true
	case "s":
		return a.latestSHA, true
	default:
		return "", false
	}
}

func replace() {
	entries, ownerRepos := collectEntries()
	if len(ownerRepos) == 0 {
		fmt.Println("No GitHub Actions found in", workflowsDir)
		return
	}

	fmt.Printf("Fetching %d repo(s)...\n", len(ownerRepos))
	checked, fetchErrs := fetchRepos(ownerRepos)
	printFetchErrors(fetchErrs)

	type key struct{ ref, tag string }
	itemMap := map[key]*replaceItem{}
	var orderedKeys []key

	for _, e := range entries {
		info := checked[e.ownerRepo]
		if info == nil {
			continue
		}
		k := key{e.actionRef, e.currentTag}
		if _, exists := itemMap[k]; !exists {
			var target string
			if hashRe.MatchString(e.currentTag) {
				target = bestTagForSHA(info, e.currentTag)
			} else {
				target = info.tags[e.currentTag]
			}
			if target == "" {
				continue
			}
			itemMap[k] = &replaceItem{
				actionRef: e.actionRef,
				current:   e.currentTag,
				target:    target,
			}
			orderedKeys = append(orderedKeys, k)
		}
		item := itemMap[k]
		if !slices.Contains(item.files, e.wfFile) {
			item.files = append(item.files, e.wfFile)
		}
	}

	if len(orderedKeys) == 0 {
		fmt.Println("\nNo convertible actions found.")
		return
	}

	sort.Slice(orderedKeys, func(i, j int) bool {
		if orderedKeys[i].ref != orderedKeys[j].ref {
			return orderedKeys[i].ref < orderedKeys[j].ref
		}
		return orderedKeys[i].tag < orderedKeys[j].tag
	})

	remaining := make([]*replaceItem, len(orderedKeys))
	for i, k := range orderedKeys {
		remaining[i] = itemMap[k]
	}

	reader := bufio.NewReader(os.Stdin)

	for len(remaining) > 0 {
		fmt.Printf("\n%s\n\n", bold(fmt.Sprintf("Actions available for conversion: %d", len(remaining))))
		for i, item := range remaining {
			if hashRe.MatchString(item.current) {
				fmt.Printf("  [%d] %s: %s %s %s  %s\n",
					i+1, bold(item.actionRef),
					cCyan(shortSHA(item.current)), cDim("→"), cGreen(item.target),
					cDim("(sha→tag)"),
				)
			} else {
				fmt.Printf("  [%d] %s: %s %s %s  %s\n",
					i+1, bold(item.actionRef),
					cYellow(item.current), cDim("→"), cCyan(shortSHA(item.target)),
					cDim("(tag→sha)"),
				)
			}
		}

		fmt.Println()
		fmt.Print("Which action to convert? (number, or q to quit): ")
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

		item := remaining[choice-1]
		remaining = append(remaining[:choice-1], remaining[choice:]...)

		var fromStr, toStr string
		if hashRe.MatchString(item.current) {
			fromStr = cCyan(shortSHA(item.current))
			toStr = cGreen(item.target)
		} else {
			fromStr = cYellow(item.current)
			toStr = cCyan(item.target)
		}
		fmt.Printf("\nConverting %s: %s %s %s\n\n", bold(item.actionRef), fromStr, cDim("→"), toStr)
		applyReplace(item.actionRef, item.current, item.target, item.files)
		fmt.Println(cGreen("\n  Done."))
	}
}
