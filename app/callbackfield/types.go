package callbackfield

// Field represents a stateful field instance that can apply itself to a request build.
type Field interface {
	// Name returns the configured field name.
	Name() string
	// Type returns the field type ("jsonValue" | "headerValue" | "queryParamValue" | "formValue").
	Type() string
	// Apply uses selected selector values to populate the request parts.
	Apply(selected map[string]string, b *RequestBuildParts)
}

// FieldPrototype is an immutable configuration object that can produce Field instances.
type FieldPrototype interface {
	NewInstance() Field
}

// RequestBuildParts accumulates request elements before constructing the http.Request.
type RequestBuildParts struct {
	// JSON is the json body map
	JSON map[string]string
	// Headers are header name -> value
	Headers map[string]string
	// Query is query parameter name -> values (to support repeated params)
	Query map[string][]string
	// Form holds form fields for multipart/form-data
	Form map[string][]string
}

// NewRequestBuildParts initializes an empty RequestBuildParts.
func NewRequestBuildParts() *RequestBuildParts {
	return &RequestBuildParts{
		JSON:    make(map[string]string),
		Headers: make(map[string]string),
		Query:   make(map[string][]string),
		Form:    make(map[string][]string),
	}
}
