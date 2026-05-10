package common

import "context"

type (
	keyTypeProperties       int
	keyTypeInterpolationMap int
)

const (
	keyProperties       keyTypeProperties       = iota
	keyInterpolationMap keyTypeInterpolationMap = iota
)

// WithProperties returns a new context carrying properties — the
// per-request property bag forwarded onto every message-bus event
// emitted while handling the request (correlation IDs, user IDs,
// etc.).
func WithProperties(ctx context.Context, properties map[string]string) context.Context {
	return withMap(ctx, keyProperties, properties)
}

// PropertiesFromContext returns the property bag previously set via
// WithProperties, or nil if none was set.
func PropertiesFromContext(ctx context.Context) map[string]string {
	return mapFromContext(ctx, keyProperties)
}

func withMap[T any](ctx context.Context, key T, value map[string]string) context.Context {
	return context.WithValue(ctx, key, value)
}

func mapFromContext[T any](ctx context.Context, key T) map[string]string {
	value := ctx.Value(key)
	if value == nil {
		return nil
	}

	if properties, ok := value.(map[string]string); ok {
		return properties
	}

	return nil
}
