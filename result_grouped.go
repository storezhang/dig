package dig

import (
	`errors`
	`reflect`

	`github.com/storezhang/dig/internal/dot`
)

var _ result = resultGrouped{}

type resultGrouped struct {
	Group   string
	Type    reflect.Type
	Flatten bool
}

func newResultGrouped(f reflect.StructField) (resultGrouped, error) {
	g, err := parseGroupString(f.Tag.Get(_groupTag))
	if err != nil {
		return resultGrouped{}, err
	}
	rg := resultGrouped{
		Group:   g.Name,
		Flatten: g.Flatten,
		Type:    f.Type,
	}
	name := f.Tag.Get(_nameTag)
	optional, _ := isFieldOptional(f)
	switch {
	case g.Flatten && f.Type.Kind() != reflect.Slice:
		return rg, errf("flatten can be applied to slices only",
			"field %q (%v) is not a slice", f.Name, f.Type)
	case name != "":
		return rg, errf(
			"cannot use named values with value groups",
			"name:%q provided with group:%q", name, rg.Group)
	case optional:
		return rg, errors.New("value groups cannot be optional")
	}
	if g.Flatten {
		rg.Type = f.Type.Elem()
	}

	return rg, nil
}

func (rt resultGrouped) DotResult() []*dot.Result {
	return []*dot.Result{{
		Node: &dot.Node{
			Type:  rt.Type,
			Group: rt.Group,
		},
	}}
}

func (rt resultGrouped) Extract(cw containerWriter, v reflect.Value) {
	if !rt.Flatten {
		cw.submitGroupedValue(rt.Group, rt.Type, v)
	} else {
		for i := 0; i < v.Len(); i++ {
			cw.submitGroupedValue(rt.Group, rt.Type, v.Index(i))
		}
	}
}
