# Contributing

Contributions are welcome — bug reports, feature requests, and pull requests.

## Reporting issues

Open an issue on GitHub. Include:
- What you ran and what you expected
- What actually happened (output or error)
- Go version (`go version`) and OS

## Development setup

```sh
git clone https://github.com/hatedabamboo/bumpwf
cd bumpwf
make build
```

No external dependencies — only the Go standard library is used.

## Submitting changes

1. Fork the repository and create a branch from `main`
2. Make your changes
3. Make sure `go build ./...` and `go vet ./...` pass
4. Open a pull request with a clear description of what and why

## Code style

- Run `gofmt` before committing
- Follow standard Go conventions (`go vet`, idiomatic error handling)
- Keep changes focused — one concern per PR

## License

By contributing you agree that your changes will be licensed under the [MIT License](LICENSE).
