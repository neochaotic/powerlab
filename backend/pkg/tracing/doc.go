// Package tracing generates and propagates correlation IDs across
// PowerLab services.
//
// One correlation ID per user action, threaded through every service
// the action touches. Logs across services join on the same ID; error
// responses include the ID so users can quote it from the UI; outbound
// HTTP calls carry the ID forward via the X-Request-Id header.
//
// Example wiring in the gateway:
//
//	mux := http.NewServeMux()
//	// ... handlers ...
//	handler := tracing.Middleware(
//	    lifecycle.RecoverMiddleware(logger)(mux),
//	)
//	http.ListenAndServe(":8443", handler)
//
// Example outbound call:
//
//	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
//	tracing.InjectHeader(req, ctx)
//	resp, err := client.Do(req)
//
// See ADR-0015 for the rationale behind the header choice and ID format.
package tracing
