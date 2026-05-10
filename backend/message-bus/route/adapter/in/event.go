// Package in adapts codegen request shapes (oapi-codegen output)
// into the internal model.* types. One adapter per type so the
// route layer never references model.* directly.
package in

import (
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
)

// EventAdapter converts a codegen.Event (REST/WS payload) into a
// model.Event for the service layer. Timestamp collapses to zero
// when unset by the publisher.
func EventAdapter(event codegen.Event) model.Event {
	// properties := make([]model.Property, 0)
	// for _, property := range  {
	// 	properties = append(properties, PropertyAdapter(property))
	// }

	var timestamp int64
	if event.Timestamp != nil {
		timestamp = event.Timestamp.Unix()
	}

	return model.Event{
		SourceID:   event.SourceID,
		Name:       event.Name,
		Properties: event.Properties,
		UUID:       *event.Uuid,

		Timestamp: timestamp,
	}
}
