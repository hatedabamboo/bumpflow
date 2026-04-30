#!/usr/bin/env bash
set -euo pipefail

REMOTE=$(git remote get-url origin 2>/dev/null | sed 's/git@github.com:/https:\/\/github.com\//; s/\.git$//')

if [[ -z "$REMOTE" ]]; then
    echo "Error: could not determine remote URL" >&2
    exit 1
fi

git log --pretty=format:"%H %ad %s" --date=short | grep -viE "\.(md)$|FUNDING" | awk -v repo="$REMOTE" '
BEGIN { prev_date = "" }
{
    sha=$1; date=$2; short=substr(sha,1,7)
    msg=""; for(i=3;i<=NF;i++) msg=(msg==""?$i:msg" "$i)
    type="chore"
    if (match(msg,/^(feat|fix|refactor|docs|test|chore|style|perf|ci|build)[(:!]/,arr)) {
        type=arr[1]; sub(/^[a-z]+(\([^)]*\))?!?:\s*/,"",msg)
    }
    if (date!=prev_date) { if(prev_date!="") print ""; print "## "date; prev_date=date }
    printf "- (%s) %s [%s](%s/commit/%s)\n", type, msg, short, repo, sha
}
'
