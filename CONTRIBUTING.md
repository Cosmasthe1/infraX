Thanks for wanting to contribute to infraX — brief guidelines to get started.

How to contribute
- Fork the repo and open a small focused PR for each change.
- Include tests for new behavior and run `go test ./...` before pushing.
- Keep commits small and scoped: one feature/fix per commit.

Development workflow
- Run the stack locally with Docker Compose:

  docker compose up --build

- Run Go unit tests:

  cd control-plane && go test ./... -v
  cd workers && go test ./... -v

- Lint with `golangci-lint` (configured in CI).

PR review checklist
- Includes tests for new behavior.
- Lints cleanly and compiles locally.
- Documentation updated if public API/behavior changed.

Code style
- Follow idiomatic Go patterns. Use `gofmt`/`go vet` where applicable.

Security & secrets
- Never commit secrets or private keys. Use environment variables or secret managers.

Contact
- Open an issue or tag the repository owners in a PR for review.
