# Contributing

Bug reports, documentation changes, tests, and code contributions are welcome. Search existing issues first and include the Go version and a small reproduction when reporting a bug. Remove secrets and real customer data from examples.

Run `go test ./...` and `go vet ./...` before submitting a pull request. Cover pending and completed verification responses when changing the client, and use exact raw bytes for webhook tests. Report security issues privately as described in [SECURITY.md](SECURITY.md).

Contributions are licensed under the [MIT License](LICENSE).
