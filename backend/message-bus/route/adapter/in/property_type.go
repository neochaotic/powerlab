package in

import (
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
)

// PropertyTypeAdapter converts a codegen.PropertyType into a
// model.PropertyType.
func PropertyTypeAdapter(propertyType codegen.PropertyType) model.PropertyType {
	return model.PropertyType{
		Name: propertyType.Name,
	}
}
