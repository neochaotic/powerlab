// Package errors is PowerLab's typed error type with code, i18n key,
// HTTP status, structured fields, and cause chain.
//
// Every PowerLab service rewritten in the CasaOS-strip emits errors of
// type *Error. The gateway middleware serializes them to a stable JSON
// shape via WriteHTTP; the UI client deserializes the same shape and
// translates the i18n key.
//
// Example:
//
//	if portInUse(8080) {
//	    return errors.ErrConflict.
//	        WithField("port", 8080).
//	        WithField("service", "nginx")
//	}
//
//	// In a handler:
//	if err := doWork(ctx); err != nil {
//	    errors.WriteHTTP(ctx, w, err)
//	    return
//	}
//
// See ADR-0013 for the rationale behind the design.
package errors
