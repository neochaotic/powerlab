package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

const (
	MessageBusAddressFilename = "message-bus.url"
	APIMessageBus             = "/v2/message_bus"
)

// EventType is a publisher-registered event schema as seen from the
// publisher side. The message-bus's own model.EventType is the
// receiver-side mirror of this; the two are kept separate so the
// SDK doesn't drag in the bus's GORM tags.
type EventType struct {
	Name             string
	SourceID         string
	PropertyTypeList []PropertyType
}

// PropertyType names a key that may appear in an event's
// Properties map. Description/Example are optional human-readable
// fields surfaced by PrintEventTypesAsMarkdown.
type PropertyType struct {
	Name        string
	Description *string
	Example     *string
}

// PrintEventTypesAsMarkdown renders the given event-type list as a
// markdown reference table (one section per type, one row per
// property). Used by the bin/ tool that emits the public event
// catalog.
func PrintEventTypesAsMarkdown(sourceID, version string, eventTypes []EventType) {
	fmt.Printf("## Source ID: `%s` (v%s)\n\n", sourceID, version)
	for _, eventType := range eventTypes {
		fmt.Printf("### Event Type: `%s`\n\n", eventType.Name)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

		fmt.Fprintln(w, "| Property\t| Description\t| Example\t|")
		fmt.Fprintln(w, "| --------\t| -----------\t| -------\t|")

		for _, propertyType := range eventType.PropertyTypeList {
			fmt.Fprintf(w, "| `%s`\t|", propertyType.Name)

			if propertyType.Description != nil {
				fmt.Fprintf(w, " %s", *propertyType.Description)
			}

			fmt.Fprintf(w, "\t|")

			if propertyType.Example != nil {
				fmt.Fprintf(w, " `%s`", *propertyType.Example)
			}

			fmt.Fprintln(w, "\t|")
		}
		w.Flush()

		fmt.Println()
	}
}

// GetMessageBusAddress returns the fully-qualified base URL for the
// message-bus REST API (.../v2/message_bus), resolved from
// message-bus.url in runtimePath.
func GetMessageBusAddress(runtimePath string) (string, error) {
	address, err := getAddress(filepath.Join(runtimePath, MessageBusAddressFilename))
	if err != nil {
		return "", err
	}

	return strings.TrimRight(address, "/") + APIMessageBus, nil
}

// PublishEventInSocket publishes an event to the message-bus over
// its unix socket (/tmp/message-bus.sock). Used by tools that don't
// have a runtime-path *.url file available — e.g. early-boot or
// migration helpers. Caller must close resp.Body if the call
// returns a non-nil response.
func PublishEventInSocket(ctx context.Context, SourceID string, Name string, properties map[string]string) (*http.Response, error) {
	socketPath := "/tmp/message-bus.sock"
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	body, err := json.Marshal(properties)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST",
		fmt.Sprintf("http://unix/v2/message_bus/event/%s/%s", SourceID, Name),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()
	return resp, nil
}
