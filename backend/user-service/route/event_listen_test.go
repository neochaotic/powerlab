package route

import (
	"strings"
	"testing"
)

// TestParseEventPayload locks in the fix for #160 (the v0.5.4
// nil-deref class in EventListen). Every input that used to panic
// must now return an error cleanly.

func TestParseEventPayload_EmptyPayload_ReturnsError(t *testing.T) {
	// On websocket disconnect, ws.Read returns 0 bytes. The original
	// code fell through to unmarshal an empty slice → unmarshal failed
	// → fell through again to *event.Uuid → panic.
	m, err := parseEventPayload([]byte{})
	if err == nil {
		t.Fatalf("expected error for empty payload, got model %+v", m)
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestParseEventPayload_MalformedJSON_ReturnsError(t *testing.T) {
	m, err := parseEventPayload([]byte("{not valid json"))
	if err == nil {
		t.Fatalf("expected error for malformed json, got model %+v", m)
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

func TestParseEventPayload_MissingUuid_ReturnsError(t *testing.T) {
	// Most important regression: payload that DOES unmarshal but lacks
	// the optional `uuid` field. This is the exact path that crashed
	// the v0.5.4 user-service every time the message-bus shut down
	// during an upgrade cycle.
	m, err := parseEventPayload([]byte(`{"name":"foo","sourceID":"bar"}`))
	if err == nil {
		t.Fatalf("expected error for missing uuid, got model %+v", m)
	}
	if !strings.Contains(err.Error(), "uuid") {
		t.Errorf("expected uuid-related error, got: %v", err)
	}
}

func TestParseEventPayload_NullUuid_ReturnsError(t *testing.T) {
	// JSON `"uuid": null` parses to a nil *string just like absence
	// of the field. Same nil-deref class.
	m, err := parseEventPayload([]byte(`{"name":"foo","sourceID":"bar","uuid":null}`))
	if err == nil {
		t.Fatalf("expected error for null uuid, got model %+v", m)
	}
}

func TestParseEventPayload_ValidPayload_ReturnsModel(t *testing.T) {
	raw := []byte(`{
		"sourceID": "local-storage",
		"name": "local-storage:disk_added",
		"uuid": "abc-123",
		"properties": {"path": "/dev/sda1", "size": "100GB"}
	}`)
	m, err := parseEventPayload(raw)
	if err != nil {
		t.Fatalf("unexpected error for valid payload: %v", err)
	}
	if m == nil {
		t.Fatal("expected model, got nil")
	}
	if m.UUID != "abc-123" {
		t.Errorf("UUID: want %q, got %q", "abc-123", m.UUID)
	}
	if m.Name != "local-storage:disk_added" {
		t.Errorf("Name: want %q, got %q", "local-storage:disk_added", m.Name)
	}
	if m.SourceID != "local-storage" {
		t.Errorf("SourceID: want %q, got %q", "local-storage", m.SourceID)
	}
	if !strings.Contains(m.Properties, "path") || !strings.Contains(m.Properties, "/dev/sda1") {
		t.Errorf("Properties: missing expected JSON content, got %q", m.Properties)
	}
}

func TestParseEventPayload_NoPanicOnAnyInput(t *testing.T) {
	// Belt-and-suspenders: fuzz the parser with a handful of crash-prone
	// inputs to make sure none of them panic. The function may return
	// an error, but it MUST NOT crash the goroutine.
	for _, raw := range [][]byte{
		nil,
		[]byte{},
		[]byte("\x00\x00\x00"),
		[]byte("[]"),
		[]byte(`""`),
		[]byte(`null`),
		[]byte(`{}`),
		[]byte(`{"uuid": ""}`),
		[]byte(`{"uuid": null, "properties": null}`),
	} {
		// If parseEventPayload panics on any of these, the test fails.
		_, _ = parseEventPayload(raw)
	}
}
