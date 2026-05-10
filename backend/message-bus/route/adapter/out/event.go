// Package out adapts internal model.* types into the codegen
// response shapes. Mirror of the in package — keeps the route
// layer free of model.* references.
package out

import (
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils"
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
)

// EventAdapter converts a model.Event into a codegen.Event for the
// response. Re-inflates the int64 unix timestamp into the codegen
// *time.Time field.
func EventAdapter(event model.Event) codegen.Event {
	// properties := make([]codegen.Property, 0)
	// for _, property := range event.Properties {
	// 	properties = append(properties, PropertyAdapter(property))
	// }

	return codegen.Event{
		SourceID:   event.SourceID,
		Name:       event.Name,
		Properties: event.Properties,
		Timestamp:  utils.Ptr(time.Unix(event.Timestamp, 0)),
		Uuid:       &event.UUID,
	}
}
