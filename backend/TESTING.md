# PowerLab Backend Testing Policy

This document outlines the testing strategy and rules for the PowerLab backend (Go microservices).

## 1. Goleak Management

We use `uber-go/goleak` to ensure no goroutine leaks are introduced in our business logic.

### Handling Third-Party Leaks
Some libraries (e.g., `ecache`, `opencensus`) start background goroutines in their `init()` functions that cannot be stopped. To prevent these from failing our tests:

1.  **Use `TestMain`:** Centralize goleak verification in a `TestMain` function for each service.
2.  **`goleak.IgnoreCurrent()`:** Use this at the beginning of `TestMain` to capture and ignore all goroutines that are already running before the tests start.
3.  **Specific Ignores:** Add `goleak.IgnoreTopFunction` for known lingering goroutines (like HTTP keep-alive loops) that might start during tests but aren't leaks in our code.

Example `TestMain` structure:
```go
func TestMain(m *testing.M) {
    opt := goleak.IgnoreCurrent()
    goleak.VerifyTestMain(m, opt,
        goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
        // ...
    )
}
```

## 2. Test Structure

*   **Unit Tests:** Located alongside the code in `*_test.go` files.
*   **Logging:** Use `logger.LogInitConsoleOnly()` in tests to avoid polluting the terminal.

## 3. Regression Testing

Always run tests for the modified service before committing:
```bash
cd backend/<service>
go test ./...
```
