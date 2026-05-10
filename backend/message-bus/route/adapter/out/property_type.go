package out

import (
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
)

// PropertyTypeAdapter converts a model.PropertyType into a
// codegen.PropertyType.
func PropertyTypeAdapter(propertyType model.PropertyType) codegen.PropertyType {
	return codegen.PropertyType{
		Name: propertyType.Name,
	}
}
