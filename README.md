```
 _                       ___ 
| |_ _ _ _____ ___ _ _ _|  _|
| . | | |     | . | | | |  _|
|___|___|_|_|_|  _|_____|_|  
              |_|            
```

# bumpwf -- Bump GitHub Actions Workflows

A CLI tool that scans `.github/workflows/` for outdated GitHub Actions and interactively (or silently) updates them.

## Install

```sh
go install github.com/hatedabamboo/bumpwf@latest
```

Or build from source:

```sh
git clone https://github.com/hatedabamboo/bumpwf
cd bumpwf
make build
make install
```

## Usage

Run from the root of a git repository:

```sh
bumpwf [options]
```

### Options

| Flag | Long form | Description |
|------|-----------|-------------|
| `-t` | `--tags` | Always use tags when updating (skips the prompt) |
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
export GH_TOKEN=ghp_...
bumpwf
```

Without a token, GitHub limits anonymous requests to **60/hour**. If you hit the limit, either set `GH_TOKEN` or use a VPN.

## Example usage

```bash
$ bumpwf
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
  [t] Tag:  v5.0.0  (committed on 2026-03-24)
  [s] SHA: cd2ce8fcbc39b97be8ca5fce6e763baed58fa128  (committed on 2026-03-24)
  [Enter] Skip
  Use tag or hash? (t/s/Enter): s

  Updated .github/workflows/pages.yaml

    Done.

Outdated action(s) remaining: 2

  [1] actions/setup-node: v5 → v6.4.0 (48b55a0)  (committed on 2026-04-20)
  [2] actions/upload-pages-artifact: v4 → v5.0.0 (fc324d3)  (committed on 2026-04-08)

Which action to update? (number, or q to quit): q
```

## License

[MIT](LICENSE)
