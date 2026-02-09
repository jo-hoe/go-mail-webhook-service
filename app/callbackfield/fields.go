package callbackfield

import (
	"log"
	"regexp"
)

// ExpandPlaceholders replaces ${selectorName} occurrences using selected values map.
// Missing selectors are substituted with empty string and logged as warnings.
func ExpandPlaceholders(input string, selected map[string]string) string {
	re := regexp.MustCompile(`\$\{([0-9A-Za-z]+)\}`)
	return re.ReplaceAllStringFunc(input, func(match string) string {
		if len(match) >= 3 {
			key := match[2 : len(match)-1]
			if val, ok := selected[key]; ok {
				return val
			}
			log.Printf("placeholder for selector '%s' not found or empty; substituting empty string", key)
		}
		return ""
	})
}

// -------------------- jsonValue --------------------

type JSONFieldPrototype struct {
	name  string
	value string
}

type JSONField struct {
	proto *JSONFieldPrototype
}

func (p *JSONFieldPrototype) NewInstance() Field {
	return &JSONField{proto: p}
}

func (f *JSONField) Name() string { return f.proto.name }
func (f *JSONField) Type() string { return "jsonValue" }

func (f *JSONField) Apply(selected map[string]string, b *RequestBuildParts) {
	if b.JSON == nil {
		b.JSON = make(map[string]string)
	}
	val := ExpandPlaceholders(f.proto.value, selected)
	b.JSON[f.proto.name] = val
}

// -------------------- headerValue --------------------

type HeaderFieldPrototype struct {
	name  string
	value string
}

type HeaderField struct {
	proto *HeaderFieldPrototype
}

func (p *HeaderFieldPrototype) NewInstance() Field {
	return &HeaderField{proto: p}
}

func (f *HeaderField) Name() string { return f.proto.name }
func (f *HeaderField) Type() string { return "headerValue" }

func (f *HeaderField) Apply(selected map[string]string, b *RequestBuildParts) {
	if b.Headers == nil {
		b.Headers = make(map[string]string)
	}
	val := ExpandPlaceholders(f.proto.value, selected)
	b.Headers[f.proto.name] = val
}

// -------------------- queryParamValue --------------------

type QueryParamFieldPrototype struct {
	name  string
	value string
}

type QueryParamField struct {
	proto *QueryParamFieldPrototype
}

func (p *QueryParamFieldPrototype) NewInstance() Field {
	return &QueryParamField{proto: p}
}

func (f *QueryParamField) Name() string { return f.proto.name }
func (f *QueryParamField) Type() string { return "queryParamValue" }

func (f *QueryParamField) Apply(selected map[string]string, b *RequestBuildParts) {
	if b.Query == nil {
		b.Query = make(map[string][]string)
	}
	val := ExpandPlaceholders(f.proto.value, selected)
	b.Query[f.proto.name] = append(b.Query[f.proto.name], val)
}