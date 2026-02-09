package callbackfield

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
