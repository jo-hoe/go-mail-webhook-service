package callbackfield

import (
	"fmt"

	"github.com/jo-hoe/go-mail-webhook-service/app/config"
)

// NewFieldPrototypes constructs immutable field prototypes from configuration.
// Supports "jsonValue", "headerValue", and "queryParamValue".
func NewFieldPrototypes(cfgs []config.CallbackField) ([]FieldPrototype, error) {
	prototypes := make([]FieldPrototype, 0, len(cfgs))
	for _, c := range cfgs {
		switch c.Type {
		case "jsonValue":
			prototypes = append(prototypes, &JSONFieldPrototype{
				name:  c.Name,
				value: c.Value,
			})
		case "headerValue":
			prototypes = append(prototypes, &HeaderFieldPrototype{
				name:  c.Name,
				value: c.Value,
			})
		case "queryParamValue":
			prototypes = append(prototypes, &QueryParamFieldPrototype{
				name:  c.Name,
				value: c.Value,
			})
		case "formValue":
			prototypes = append(prototypes, &FormValueFieldPrototype{
				name:  c.Name,
				value: c.Value,
			})
		default:
			return nil, fmt.Errorf("unsupported callback field type '%s' for field '%s'", c.Type, c.Name)
		}
	}
	return prototypes, nil
}