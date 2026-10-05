# PowerLab Backend Testing Policy

This document outlines the testing strategy and rules for the PowerLab backend (Go microservices).

## 1. Goleak Management

We use `uber-go/goleak` to ensure no goroutine leaks are introduced in our business logic.

### Handling Third-Party Leaks
Some libraries (e.g., `ecache`, `opencensus`) start background goroutines in their `init()` functions that cannot be stopped. To prevent these from failing our tests:

1.  **Use `TestMain`:** Centralize goleak verification in a `TestMain` function for each service.
2.  **Use the shared helper:** `backend/common/utils/testutil` (every service module already requires `backend/common`). `testutil.VerifyTestMain(m, opts...)` wraps `goleak.VerifyTestMain` and adds `goleak.IgnoreCurrent()`, so goroutines already running before the tests start are ignored.
3.  **Specific Ignores:** Reuse the shared options for known lingering goroutines instead of copying `goleak.IgnoreTopFunction` strings: `testutil.HTTPClientIgnores()` (HTTP keep-alive loops), `testutil.OpenCensusIgnore()`, `testutil.SocketIOIgnore()`. Add a new shared option there when a second package needs the same ignore.

Example `TestMain` structure:
```go
func TestMain(m *testing.M) {
    testutil.VerifyTestMain(m, testutil.HTTPClientIgnores()...)
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
