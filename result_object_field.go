package dig

import (
	`reflect`

	`github.com/storezhang/dig/internal/dot`
)

type resultObjectField struct {
	// Name of the field in the struct.
	FieldName string

	// Index of the field in the struct.
	//
	// We need to track this separately because not all fields of the struct
	// map to results.
	FieldIndex int

	// Result produced by this field.
	Result result
}

func (rof resultObjectField) DotResult() []*dot.Result {
	return rof.Result.DotResult()
}

// newResultObjectField(i, f, opts) builds a resultObjectField from the field
// f at index i.
func newResultObjectField(idx int, f reflect.StructField, opts resultOptions) (resultObjectField, error) {
	rof := resultObjectField{
		FieldName:  f.Name,
		FieldIndex: idx,
	}

	var r result
	switch {
	case f.PkgPath != "":
		return rof, errf(
			"unexported fields not allowed in dig.Out, did you mean to export %q (%v)?", f.Name, f.Type)

	case f.Tag.Get(_groupTag) != "":
		var err error
		r, err = newResultGrouped(f)
		if err != nil {
			return rof, err
		}

	default:
		var err error
		if name := f.Tag.Get(_nameTag); len(name) > 0 {
			// can modify in-place because options are passed-by-value.
			opts.Name = name
		}
		r, err = newResult(f.Type, opts)
		if err != nil {
			return rof, err
		}
	}

	rof.Result = r
	return rof, nil
}
