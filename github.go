package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
)

var ghToken = os.Getenv("GH_TOKEN")

var apiBaseURL = "https://api.github.com"

type tagInfo struct {
	tag  string
	sha  string
	date string
}

type repoInfo struct {
	latest tagInfo
	tags   map[string]string // tag name -> sha
}

func githubGet(url string) ([]byte, error) {
	slog.Debug("HTTP request", "method", "GET", "url", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "update-github-actions/1.0")
	req.Header.Set("Accept", "application/vnd.github+json")
	if ghToken != "" {
		req.Header.Set("Authorization", "Bearer "+ghToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Debug("HTTP request failed", "url", url, "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	slog.Debug("HTTP response", "url", url, "status", resp.StatusCode)
	if resp.StatusCode == http.StatusForbidden && ghToken == "" {
		return nil, fmt.Errorf("rate limited: anonymous GitHub API requests are limited to 60/hour — set GH_TOKEN or use a VPN")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func getCommitDate(ownerRepo, sha string) string {
	data, err := githubGet(fmt.Sprintf("%s/repos/%s/commits/%s", apiBaseURL, ownerRepo, sha))
	if err != nil {
		slog.Debug("failed to get commit date", "repo", ownerRepo, "sha", sha, "error", err)
		return "unknown"
	}
	var result struct {
		Commit struct {
			Committer struct {
				Date string `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		slog.Debug("failed to parse commit date response", "repo", ownerRepo, "sha", sha, "error", err)
		return "unknown"
	}
	if len(result.Commit.Committer.Date) >= 10 {
		return result.Commit.Committer.Date[:10]
	}
	return "unknown"
}

func getRepoTagInfo(ownerRepo string) (*repoInfo, error) {
	data, err := githubGet(fmt.Sprintf("%s/repos/%s/tags?per_page=100", apiBaseURL, ownerRepo))
	if err != nil {
		return nil, err
	}
	var tags []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil, err
	}
	slog.Debug("fetched tags", "repo", ownerRepo, "count", len(tags))

	allTags := make(map[string]string, len(tags))
	var bestTag, bestSHA string
	for _, t := range tags {
		allTags[t.Name] = t.Commit.SHA
		if !semverRe.MatchString(t.Name) {
			continue
		}
		if bestTag == "" || versionGreater(t.Name, bestTag) {
			bestTag = t.Name
			bestSHA = t.Commit.SHA
		}
	}
	if bestTag == "" {
		return nil, nil
	}
	return &repoInfo{
		latest: tagInfo{tag: bestTag, sha: bestSHA, date: getCommitDate(ownerRepo, bestSHA)},
		tags:   allTags,
	}, nil
}

func bestTagForSHA(info *repoInfo, sha string) string {
	var best string
	for tag, s := range info.tags {
		if s != sha || !semverRe.MatchString(tag) {
			continue
		}
		if best == "" || versionGreater(tag, best) {
			best = tag
		}
	}
	return best
}

func fetchRepos(ownerRepos []string) (map[string]*repoInfo, map[string]error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	checked := make(map[string]*repoInfo, len(ownerRepos))
	errs := make(map[string]error)

	for _, ownerRepo := range ownerRepos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			slog.Debug("fetching repo tags", "repo", repo)
			info, err := getRepoTagInfo(repo)
			mu.Lock()
			checked[repo] = info
			if err != nil {
				slog.Debug("failed to fetch repo tags", "repo", repo, "error", err)
				errs[repo] = err
			}
			mu.Unlock()
		}(ownerRepo)
	}
	wg.Wait()
	return checked, errs
}
