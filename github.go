package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultAPIBaseURL = "https://api.github.com"
const maxTagPages = 10

var ghToken = os.Getenv("GH_TOKEN")

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
	data, _, err := githubGetPage(url)
	return data, err
}

func githubGetPage(url string) ([]byte, string, error) {
	slog.Debug("HTTP request", "method", "GET", "url", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "bumpflow/"+version)
	req.Header.Set("Accept", "application/vnd.github+json")
	if ghToken != "" {
		req.Header.Set("Authorization", "Bearer "+ghToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Debug("HTTP request failed", "url", url, "error", err)
		return nil, "", err
	}
	defer resp.Body.Close()
	slog.Debug("HTTP response", "url", url, "status", resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusOK:
		data, err := io.ReadAll(resp.Body)
		return data, nextLink(resp.Header.Get("Link")), err
	case http.StatusUnauthorized:
		return nil, "", fmt.Errorf("GitHub authentication failed: GH_TOKEN is invalid or has been revoked — generate a new token at https://github.com/settings/tokens")
	case http.StatusForbidden:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			if ts, err2 := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err2 == nil {
				return nil, "", fmt.Errorf("GitHub API rate limit exceeded — resets at %s", time.Unix(ts, 0).Local().Format("15:04 MST"))
			}
			return nil, "", fmt.Errorf("GitHub API rate limit exceeded")
		}
		if ghToken == "" {
			return nil, "", fmt.Errorf("rate limited: anonymous GitHub API requests are limited to 60/hour — set GH_TOKEN or use a VPN")
		}
		return nil, "", fmt.Errorf("GitHub API access forbidden: verify GH_TOKEN has 'public_repo' or 'repo' scope")
	case http.StatusNotFound:
		return nil, "", fmt.Errorf("repository not found — check if it's private or the action name is correct (HTTP 404)")
	default:
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
}

func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		segs := strings.Split(part, ";")
		if len(segs) < 2 {
			continue
		}
		u := strings.TrimSpace(segs[0])
		if !strings.HasPrefix(u, "<") || !strings.HasSuffix(u, ">") {
			continue
		}
		for _, p := range segs[1:] {
			if strings.TrimSpace(p) == `rel="next"` {
				return u[1 : len(u)-1]
			}
		}
	}
	return ""
}

func sameOrigin(u, base string) bool {
	pu, err1 := neturl.Parse(u)
	pb, err2 := neturl.Parse(base)
	return err1 == nil && err2 == nil && pu.Scheme == pb.Scheme && pu.Host == pb.Host
}

func getCommitDate(ownerRepo, sha, baseURL string) string {
	data, err := githubGet(fmt.Sprintf("%s/repos/%s/commits/%s", baseURL, ownerRepo, sha))
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

type ghTag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

func fetchTags(ownerRepo, baseURL string) ([]ghTag, error) {
	var tags []ghTag
	url := fmt.Sprintf("%s/repos/%s/tags?per_page=100", baseURL, ownerRepo)
	for page := 0; url != ""; page++ {
		if page == maxTagPages {
			slog.Debug("tag page cap reached, older tags ignored", "repo", ownerRepo, "pages", maxTagPages, "tags", len(tags))
			break
		}
		data, next, err := githubGetPage(url)
		if err != nil {
			return nil, fmt.Errorf("fetching tags for %s: %w", ownerRepo, err)
		}
		var pageTags []ghTag
		if err := json.Unmarshal(data, &pageTags); err != nil {
			return nil, fmt.Errorf("parsing tags for %s: %w", ownerRepo, err)
		}
		tags = append(tags, pageTags...)
		if next != "" && !sameOrigin(next, baseURL) {
			slog.Debug("ignoring pagination link to another host", "repo", ownerRepo, "url", next)
			next = ""
		}
		url = next
	}
	return tags, nil
}

func getRepoTagInfo(ownerRepo, baseURL string, count int) (*repoInfo, error) {
	tags, err := fetchTags(ownerRepo, baseURL)
	if err != nil {
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
	topTags := semverTags[:count]

	return &repoInfo{
		latest:  topTags[0],
		topTags: topTags,
		tags:    allTags,
	}, nil
}

func bestTagForSHA(info *repoInfo, sha string) string {
	var best string
	var bestParts []int
	for tag, s := range info.tags {
		if s != sha || !semverRe.MatchString(tag) {
			continue
		}
		parts, ok := parseVersion(tag)
		if !ok {
			continue
		}
		if best == "" {
			best = tag
			bestParts = parts
			continue
		}
		if versionGreater(tag, best) || (len(parts) > len(bestParts) && !versionGreater(best, tag)) {
			best = tag
			bestParts = parts
		}
	}
	return best
}

func fetchRepos(ownerRepos []string, count int, baseURL string, cache *cacheFile, cacheAge time.Duration) (map[string]*repoInfo, map[string]error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	checked := make(map[string]*repoInfo, len(ownerRepos))
	errs := make(map[string]error)

	for _, ownerRepo := range ownerRepos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()

			mu.Lock()
			if cache != nil {
				if info, ok := cache.get(repo, count, cacheAge); ok {
					slog.Debug("cache hit", "repo", repo)
					checked[repo] = info
					mu.Unlock()
					return
				}
				slog.Debug("cache miss", "repo", repo)
			}
			mu.Unlock()

			slog.Debug("fetching repo tags", "repo", repo)
			info, err := getRepoTagInfo(repo, baseURL, count)

			mu.Lock()
			checked[repo] = info
			if err != nil {
				slog.Debug("failed to fetch repo tags", "repo", repo, "error", err)
				errs[repo] = err
			} else if cache != nil && info != nil {
				cache.set(repo, info)
			}
			mu.Unlock()
		}(ownerRepo)
	}
	wg.Wait()
	return checked, errs
}

func fillLatestDates(actions []action, checked map[string]*repoInfo, baseURL string, cache *cacheFile) {
	missing := map[string]*repoInfo{}
	for _, a := range actions {
		if info := checked[a.ownerRepo]; info != nil && info.latest.date == "" {
			missing[a.ownerRepo] = info
		}
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for repo, info := range missing {
		wg.Add(1)
		go func(repo string, info *repoInfo) {
			defer wg.Done()
			date := getCommitDate(repo, info.latest.sha, baseURL)

			mu.Lock()
			defer mu.Unlock()
			info.latest.date = date
			if len(info.topTags) > 0 && info.topTags[0].sha == info.latest.sha {
				info.topTags[0].date = date
			}
			if cache != nil && date != "unknown" {
				cache.setDate(repo, date)
			}
		}(repo, info)
	}
	wg.Wait()
	for i := range actions {
		if info := checked[actions[i].ownerRepo]; info != nil {
			actions[i].latestDate = info.latest.date
		}
	}
}
