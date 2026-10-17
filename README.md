# MailCheckr for Go

Server-side Go client for the [MailCheckr API](https://mailcheckr.app/docs). It uses only the Go standard library.

## Install

```sh
go get github.com/dotmarn/mailcheckr-go
```

Set `MAILCHECKR_API_KEY` on your server and keep it out of client-side code.

## Verify an email

```go
package main

import (
    "context"
    "log"
    "os"

    mailcheckr "github.com/dotmarn/mailcheckr-go"
)

func main() {
    client, err := mailcheckr.NewClient(os.Getenv("MAILCHECKR_API_KEY"))
    if err != nil { log.Fatal(err) }

    result, err := client.Verify(context.Background(), "person@example.com", "verify-user-123")
    if err != nil { log.Fatal(err) }

    if result.IsPending() {
        // Save result.ID and poll later, or handle a webhook.
        result, err = client.Find(context.Background(), result.ID)
        if err != nil { log.Fatal(err) }
    }

    if result.IsDeliverable() {
        log.Println("Address is deliverable")
    }
}
```

Use a stable idempotency key for each verification, and reuse it when retrying the same email. Reusing a key with a different email receives HTTP 409. `Verify` accepts both HTTP 200 completed and HTTP 202 pending results. Pending states are `queued`, `processing`, and `retry_scheduled`; completed statuses are `deliverable`, `undeliverable`, `risky`, and `unknown`. Only a completed `deliverable` result passes `IsDeliverable()`.

`NewClient` accepts `WithBaseURL` and `WithHTTPClient` options. The default base URL is `https://mailcheckr.app/api/v1`; the default HTTP timeout is 30 seconds. `Verification.Raw` contains the complete API data object, including fields added later. API failures return `*mailcheckr.APIError` with `StatusCode` and `Response`; network and timeout failures return the underlying Go error.

## Verify webhooks

Verify the exact raw body before parsing JSON or applying the event:

```go
verifier := mailcheckr.NewWebhookVerifier(os.Getenv("MAILCHECKR_WEBHOOK_SECRET"))

http.HandleFunc("/webhooks/mailcheckr", func(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
    if err != nil || !verifier.Verify(body, r.Header) {
        http.Error(w, "invalid webhook", http.StatusUnauthorized)
        return
    }
    // Parse body, then persist and deduplicate the event ID before applying effects.
    w.WriteHeader(http.StatusNoContent)
})
```

The verifier checks the `X-MailCheckr-Signature` HMAC SHA-256 over `X-MailCheckr-Timestamp + "." + raw body` and rejects timestamps outside a five-minute window. Pass a positive `time.Duration` to `NewWebhookVerifier(secret, tolerance)` to customize the window. Valid deliveries can be retried, so deduplicate event IDs in persistent storage.

## Development

```sh
go test ./...
go vet ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).

## License

MIT
