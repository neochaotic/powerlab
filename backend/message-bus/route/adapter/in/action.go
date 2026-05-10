package in

import (
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
)

// ActionAdapter converts a codegen.Action (REST/WS payload) into a
// model.Action for the service layer.
func ActionAdapter(action codegen.Action) model.Action {
	var timestamp int64
	if action.Timestamp != nil {
		timestamp = action.Timestamp.Unix()
	}

	return model.Action{
		SourceID:   action.SourceID,
		Name:       action.Name,
		Properties: action.Properties,
		Timestamp:  timestamp,
	}
}
