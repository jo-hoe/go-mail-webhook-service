package callbackfield

type FormValueFieldPrototype struct {
	name  string
	value string
}

type FormValueField struct {
	proto *FormValueFieldPrototype
}

func (p *FormValueFieldPrototype) NewInstance() Field {
	return &FormValueField{proto: p}
}

func (f *FormValueField) Name() string { return f.proto.name }
func (f *FormValueField) Type() string { return "formValue" }

func (f *FormValueField) Apply(selected map[string]string, b *RequestBuildParts) {
	if b.Form == nil {
		b.Form = make(map[string][]string)
	}
	val := ExpandPlaceholders(f.proto.value, selected)
	// Single value semantics
	b.Form[f.proto.name] = append(b.Form[f.proto.name], val)
}