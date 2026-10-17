# Contributing to MailCheckr for Go

Thanks for helping improve the Go client. Bug reports, documentation fixes, tests, and code contributions are welcome.

## Before you start

- Search existing issues and pull requests to see whether the problem is already being discussed.
- For a substantial feature, public API change, or new dependency, open an issue first so the approach can be agreed on before implementation.
- Report suspected vulnerabilities privately as described in [SECURITY.md](SECURITY.md). Do not put exploit details or secrets in a public issue.

When reporting a bug, include your Go version, the package version or commit, the expected and actual behavior, and a small reproduction. Remove API keys, webhook secrets, and real customer data from examples.

## Set up locally

Install Go 1.21 or newer, fork the repository, and clone your fork. This package uses only the Go standard library, so it needs no service account or external dependencies for local tests.

```sh
git clone https://github.com/YOUR_USERNAME/mailcheckr-go.git
cd mailcheckr-go
go test ./...
go vet ./...
```

Create a branch for your change. Keep each pull request focused on one issue or feature.

## Make a change

- Keep the public API and behavior consistent with the [MailCheckr API](https://mailcheckr.app/docs). Preserve idempotency keys and the distinction between pending, failed, and completed verification results.
- Add or update tests for behavior changes. Cover both HTTP 200 completed and HTTP 202 pending responses when relevant. Use an in-memory HTTP transport so tests do not need a live MailCheckr account or network access.
- For webhook changes, test signatures against the exact raw body bytes and include invalid signature and timestamp cases.
- Update the README and exported Go documentation when a public API or usage pattern changes.
- Avoid adding dependencies without discussing the need in an issue first.

Format and check your work before opening a pull request:

```sh
gofmt -w .
go test ./...
go vet ./...
git diff --check
```

## Open a pull request

Describe the problem, the change, and how you tested it. Link the related issue when there is one. If you changed public behavior, include a short usage example and note any compatibility impact. Maintainers may ask for changes during review; keep discussion constructive and focused on the code.

By contributing, you agree that your contribution is licensed under the repository's [MIT License](LICENSE).
