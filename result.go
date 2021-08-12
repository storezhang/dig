package dig

import (
	`fmt`
	`reflect`

	`github.com/storezhang/dig/internal/dot`
)

type result interface {
	Extract(containerWriter, reflect.Value)
	DotResult() []*dot.Result
}

type resultOptions struct {
	// If set, this is the name of the associated result value.
	//
	// For Result Objects, name:".." tags on fields override this.
	Name  string
	Group string
}

// newResult builds a result from the given type.
func newResult(t reflect.Type, opts resultOptions) (result, error) {
	switch {
	case isIn(t) || (t.Kind() == reflect.Ptr && isIn(t.Elem())) || isEmbed(t, _inPtrType):
		return nil, errf("cannot provide parameter objects", "%v embeds a dig.In", t)
	case isError(t):
		return nil, errf("cannot return an error here, return it from the constructor instead")
	case isOut(t):
		return newResultObject(t, opts)
	case isEmbed(t, _outPtrType):
		return nil, errf(
			"cannot build a result object by embedding *dig.Out, embed dig.Out instead",
			"%v embeds *dig.Out", t)
	case t.Kind() == reflect.Ptr && isOut(t.Elem()):
		return nil, errf(
			"cannot return a pointer to a result object, use a value instead",
			"%v is a pointer to a struct that embeds dig.Out", t)
	case len(opts.Group) > 0:
		g, err := parseGroupString(opts.Group)
		if err != nil {
			return nil, errf(
				"cannot parse group %q", opts.Group, err)
		}
		rg := resultGrouped{Type: t, Group: g.Name, Flatten: g.Flatten}
		if g.Flatten {
			if t.Kind() != reflect.Slice {
				return nil, errf(
					"flatten can be applied to slices only",
					"%v is not a slice", t)
			}
			rg.Type = rg.Type.Elem()
		}
		return rg, nil
	default:
		return resultSingle{Type: t, Name: opts.Name}, nil
	}
}

func walkResult(r result, v resultVisitor) {
	v = v.Visit(r)
	if v == nil {
		return
	}

	switch res := r.(type) {
	case resultSingle, resultGrouped:
		// No sub-results
	case resultObject:
		w := v
		for _, f := range res.Fields {
			if v := w.AnnotateWithField(f); v != nil {
				walkResult(f.Result, v)
			}
		}
	case resultList:
		w := v
		for i, r := range res.Results {
			if v := w.AnnotateWithPosition(i); v != nil {
				walkResult(r, v)
			}
		}
	default:
		panic(fmt.Sprintf(
			"It looks like you have found a bug in dig. "+
				"Please file an issue at https://github.com/uber-go/dig/issues/ "+
				"and provide the following message: "+
				"received unknown result type %T", res))
	}
}
