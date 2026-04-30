package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
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
	latest  tagInfo
	topTags []tagInfo
	tags    map[string]string // tag name -> sha
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

func getRepoTagInfo(ownerRepo string, count int) (*repoInfo, error) {
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
	var semverTags []tagInfo

	for _, t := range tags {
		allTags[t.Name] = t.Commit.SHA
		if semverRe.MatchString(t.Name) {
			semverTags = append(semverTags, tagInfo{tag: t.Name, sha: t.Commit.SHA})
		}
	}

	if len(semverTags) == 0 {
		return nil, nil
	}

	sort.Slice(semverTags, func(i, j int) bool {
		return versionGreater(semverTags[i].tag, semverTags[j].tag)
	})

	if count > len(semverTags) {
		count = len(semverTags)
	}
	semverTags = semverTags[:count]

	topTags := semverTags
	topTags[0].date = getCommitDate(ownerRepo, topTags[0].sha)

	return &repoInfo{
		latest:  topTags[0],
		topTags: topTags,
		tags:    allTags,
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

func fetchRepos(ownerRepos []string, count int) (map[string]*repoInfo, map[string]error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	checked := make(map[string]*repoInfo, len(ownerRepos))
	errs := make(map[string]error)

	for _, ownerRepo := range ownerRepos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			slog.Debug("fetching repo tags", "repo", repo)
			info, err := getRepoTagInfo(repo, count)
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
