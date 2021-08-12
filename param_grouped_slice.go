package dig

import (
	`errors`
	`fmt`
	`reflect`

	`github.com/storezhang/dig/internal/dot`
)

type paramGroupedSlice struct {
	Group string
	Type  reflect.Type
}

func (pt paramGroupedSlice) DotParam() []*dot.Param {
	return []*dot.Param{
		{
			Node: &dot.Node{
				Type:  pt.Type,
				Group: pt.Group,
			},
		},
	}
}
func newParamGroupedSlice(f reflect.StructField) (paramGroupedSlice, error) {
	g, err := parseGroupString(f.Tag.Get(_groupTag))
	if err != nil {
		return paramGroupedSlice{}, err
	}
	pg := paramGroupedSlice{Group: g.Name, Type: f.Type}

	name := f.Tag.Get(_nameTag)
	optional, _ := isFieldOptional(f)
	switch {
	case f.Type.Kind() != reflect.Slice:
		return pg, errf("value groups may be consumed as slices only",
			"field %q (%v) is not a slice", f.Name, f.Type)
	case g.Flatten:
		return pg, errf("cannot use flatten in parameter value groups",
			"field %q (%v) specifies flatten", f.Name, f.Type)
	case name != "":
		return pg, errf(
			"cannot use named values with value groups",
			"name:%q requested with group:%q", name, pg.Group)

	case optional:
		return pg, errors.New("value groups cannot be optional")
	}

	return pg, nil
}

func (pt paramGroupedSlice) Build(c containerStore) (reflect.Value, error) {
	for _, n := range c.getGroupProviders(pt.Group, pt.Type.Elem()) {
		if err := n.Call(c); err != nil {
			return _noValue, errParamGroupFailed{
				CtorID: n.ID(),
				Key:    key{group: pt.Group, t: pt.Type.Elem()},
				Reason: err,
			}
		}
	}

	items := c.getValueGroup(pt.Group, pt.Type.Elem())

	result := reflect.MakeSlice(pt.Type, len(items), len(items))
	for i, v := range items {
		result.Index(i).Set(v)
	}
	return result, nil
}

func (pt paramGroupedSlice) String() string {
	return fmt.Sprintf("%v[group=%q]", pt.Type.Elem(), pt.Group)
}
