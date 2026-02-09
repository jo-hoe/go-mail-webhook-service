package callbackfield

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