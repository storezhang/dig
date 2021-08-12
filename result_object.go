package dig

import (
	`reflect`

	`github.com/storezhang/dig/internal/dot`
)

var _ result = resultObject{}

type resultObject struct {
	Type   reflect.Type
	Fields []resultObjectField
}

func (ro resultObject) DotResult() []*dot.Result {
	var types []*dot.Result
	for _, field := range ro.Fields {
		types = append(types, field.DotResult()...)
	}
	return types
}

func newResultObject(t reflect.Type, opts resultOptions) (resultObject, error) {
	ro := resultObject{Type: t}
	if len(opts.Name) > 0 {
		return ro, errf(
			"cannot specify a name for result objects", "%v embeds dig.Out", t)
	}

	if len(opts.Group) > 0 {
		return ro, errf(
			"cannot specify a group for result objects", "%v embeds dig.Out", t)
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == _outType {
			// Skip over the dig.Out embed.
			continue
		}

		rof, err := newResultObjectField(i, f, opts)
		if err != nil {
			return ro, errf("bad field %q of %v", f.Name, t, err)
		}

		ro.Fields = append(ro.Fields, rof)
	}
	return ro, nil
}

func (ro resultObject) Extract(cw containerWriter, v reflect.Value) {
	for _, f := range ro.Fields {
		f.Result.Extract(cw, v.Field(f.FieldIndex))
	}
}
