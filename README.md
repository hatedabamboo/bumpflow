```
 _                       ___ 
| |_ _ _ _____ ___ _ _ _|  _|
| . | | |     | . | | | |  _|
|___|___|_|_|_|  _|_____|_|  
              |_|            
```

# [bumpflow](https://notes.hatedabamboo.me/project-bumpflow/) — Bump GitHub Actions Workflows

A CLI tool that scans `.github/workflows/` for outdated GitHub Actions and interactively (or silently) updates them. Also can replace current version (`actions/checkout@v4`) with the commit tag (`34e114876b0b11c390a56381ad16ebd13914f8d5`). Prevent those pesky clawed clankers from hijacking your workflows!

## Install

```sh
go install github.com/hatedabamboo/bumpflow@latest
```

Or build from source:

```sh
git clone https://github.com/hatedabamboo/bumpflow
cd bumpflow
make
make install
```

## Usage

Run from the root of a git repository:

```sh
bumpflow [options]
```

### Options

| Flag | Long form | Description |
|------|-----------|-------------|
| `-t` | `--tags` | Always use tags when updating (skips the prompt) |
| `-n` | `--count` | Number of latest tags to fetch (default 10) |
| `-s` | `--sha` | Always use commit hashes when updating (skips the prompt) |
| `-A` | `--update-all` | Update all outdated actions without prompting (uses hash by default; respects `-t` or `-s`) |
| `-r` | `--replace` | Convert pinned tags↔SHAs without upgrading versions |
| `-v` | `--verbose` | Enable verbose logging |
| `-V` | `--version` | Show version |
| `-h` | `--help` | Show usage |

`-t` and `-s` are mutually exclusive. `-A` and `-r` are mutually exclusive.

## Authentication

Set `GH_TOKEN` to a GitHub personal access token for authenticated API calls:

```sh
export GH_TOKEN="ghp_..."
bumpflow
```

Without a token, GitHub limits anonymous requests to 60/hour. If you hit the limit, either set `GH_TOKEN` or use a VPN.

## Example workflow usage

To utilize `bumpflow` capabilities in automatic fashion, you must create Fine-grained [Personal Access Token](https://github.com/settings/personal-access-tokens/new) with the following settings:

- **Name**: `WORKFLOW_TOKEN` (can be any)
- **Expiration**: any (recommended to set to 90 days)
- **Repository access**: select only repository you want to update the workflows in
- **Permissions**:
  - Contents: Read and write
  - Metadata (Required): Read-only
  - Pull requests: Read and write
  - Workflows: Read and write

Add the token to the repository secrets under the name `WORKFLOW_TOKEN` (can be any, but this name is used in the example). Copy the example workflow and save it under `.github/workflows/upgrade-workflows.yaml` (or any name you like).

```yaml
---
name: Upgrade workflows

on:
  schedule:
    - cron: "0 9 * * 1" # trigger every Monday at 9:00 UTC
  workflow_dispatch:    # or manually

permissions:
  contents: write
  pull-requests: write

jobs:
  upgrade:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd
        with:
          # necessary to not conflict with peter-evans/create-pull-request action
          persist-credentials: false

      - name: Setup Go
        uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c
        with:
          go-version: stable

      - name: Install bumpflow
        run: go install github.com/hatedabamboo/bumpflow@latest

      - name: Upgrade workflows
        run: |
          "$(go env GOPATH)/bin/bumpflow" -A # upgrade all automatically

      - name: Create labels
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          gh label create chore --color bfdadc --force
          gh label create dependencies --color bfd4f2 --force
          gh label create automation --color fef2c0 --force

      - name: Create pull request
        uses: peter-evans/create-pull-request@5f6978faf089d4d20b00c7766989d076bb2fc7f1
        with:
          token: ${{ secrets.WORKFLOW_TOKEN }}
          commit-message: "chore: upgrade workflow action versions"
          branch: chore/upgrade-workflow-actions
          delete-branch: true
          title: "chore: upgrade workflow action versions"
          body: |
            Automated upgrade of GitHub Actions versions by [bumpflow](https://github.com/hatedabamboo/bumpflow).
          labels: chore,dependencies,automation
```

## Example interactive usage

```bash
$ bumpflow
Fetching 4 repo(s)...

  Action                                        Installed version              Latest version
  ------                                        -----------------              --------------
  actions/checkout                              de0fac2                        v6.0.2 (de0fac2)
  actions/deploy-pages                          v4                             v5.0.0 (cd2ce8f)
  actions/setup-node                            v5                             v6.4.0 (48b55a0)
  actions/upload-pages-artifact                 v4                             v5.0.0 (fc324d3)

Outdated action(s) remaining: 3

  [1] actions/deploy-pages: v4 → v5.0.0 (cd2ce8f)  (committed on 2026-03-24)
  [2] actions/setup-node: v5 → v6.4.0 (48b55a0)  (committed on 2026-04-20)
  [3] actions/upload-pages-artifact: v4 → v5.0.0 (fc324d3)  (committed on 2026-04-08)

Which action to update? (number, or q to quit): 1

Updating actions/deploy-pages:
  [1]		v5.0.0	(cd2ce8f)
  [2]		v4.0.0	(1e31a15)
  [Enter]	Skip
  Select version (number): 1
  [t] Tag:  v5.0.0
  [s] SHA:  cd2ce8fcbc39b97be8ca5fce6e763baed58fa128
  Use tag or hash? (t/s): s

  Updated .github/workflows/pages.yaml

    Done.

Outdated action(s) remaining: 2

  [1] actions/setup-node: v5 → v6.4.0 (48b55a0)  (committed on 2026-04-20)
  [2] actions/upload-pages-artifact: v4 → v5.0.0 (fc324d3)  (committed on 2026-04-08)

Which action to update? (number, or q to quit): q
```
