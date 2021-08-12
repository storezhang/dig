package dig

import (
	`reflect`
	`strings`

	`github.com/storezhang/dig/internal/dot`
)

type paramObject struct {
	Type   reflect.Type
	Fields []paramObjectField
}

func newParamObject(t reflect.Type) (paramObject, error) {
	po := paramObject{Type: t}

	// Check if the In type supports ignoring unexported fields.
	var ignoreUnexported bool
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == _inType {
			var err error
			ignoreUnexported, err = isIgnoreUnexportedSet(f)
			if err != nil {
				return po, err
			}
			break
		}
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == _inType {
			// Skip over the dig.In embed.
			continue
		}
		if f.PkgPath != "" && ignoreUnexported {
			// Skip over an unexported field if it is allowed.
			continue
		}
		pof, err := newParamObjectField(i, f)
		if err != nil {
			return po, errf("bad field %q of %v", f.Name, t, err)
		}

		po.Fields = append(po.Fields, pof)
	}

	return po, nil
}

func (po paramObject) DotParam() []*dot.Param {
	var types []*dot.Param
	for _, field := range po.Fields {
		types = append(types, field.DotParam()...)
	}
	return types
}

func (op paramObject) String() string {
	fields := make([]string, len(op.Fields))
	for i, f := range op.Fields {
		fields[i] = f.Param.String()
	}
	return strings.Join(fields, " ")
}

func (po paramObject) Build(c containerStore) (reflect.Value, error) {
	dest := reflect.New(po.Type).Elem()
	for _, f := range po.Fields {
		v, err := f.Build(c)
		if err != nil {
			return dest, err
		}
		dest.Field(f.FieldIndex).Set(v)
	}
	return dest, nil
}
