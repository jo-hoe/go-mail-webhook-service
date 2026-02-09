package callbackfield

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
