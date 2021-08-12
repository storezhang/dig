package dig

import (
	"fmt"
	"reflect"

	"github.com/storezhang/dig/internal/dot"
)

type param interface {
	fmt.Stringer

	Build(containerStore) (reflect.Value, error)

	// DotParam returns a slice of dot.Param(s).
	DotParam() []*dot.Param
}

var (
	_ param = paramSingle{}
	_ param = paramObject{}
	_ param = paramList{}
	_ param = paramGroupedSlice{}
)

// newParam builds a param from the given type. If the provided type is a
// dig.In struct, an paramObject will be returned.
func newParam(t reflect.Type) (param, error) {
	switch {
	case isOut(t) || (t.Kind() == reflect.Ptr && isOut(t.Elem())) || isEmbed(t, _outPtrType):
		return nil, errf("cannot depend on result objects", "%v embeds a dig.Out", t)
	case isIn(t):
		return newParamObject(t)
	case isEmbed(t, _inPtrType):
		return nil, errf(
			"cannot build a parameter object by embedding *dig.In, embed dig.In instead",
			"%v embeds *dig.In", t)
	case t.Kind() == reflect.Ptr && isIn(t.Elem()):
		return nil, errf(
			"cannot depend on a pointer to a parameter object, use a value instead",
			"%v is a pointer to a struct that embeds dig.In", t)
	default:
		return paramSingle{Type: t}, nil
	}
}

// paramVisitor visits every param in a param tree, allowing tracking state at
// each level.
type paramVisitor interface {
	// Visit is called on the param being visited.
	//
	// If Visit returns a non-nil paramVisitor, that paramVisitor visits all
	// the child params of this param.
	Visit(param) paramVisitor

	// We can implement AnnotateWithField and AnnotateWithPosition like
	// resultVisitor if we need to track that information in the future.
}

// paramVisitorFunc is a paramVisitor that visits param in a tree with the
// return value deciding whether the descendants of this param should be
// recursed into.
type paramVisitorFunc func(param) (recurse bool)

func (f paramVisitorFunc) Visit(p param) paramVisitor {
	if f(p) {
		return f
	}
	return nil
}

// walkParam walks the param tree for the given param with the provided
// visitor.
//
// paramVisitor.Visit will be called on the provided param and if a non-nil
// paramVisitor is received, this param's descendants will be walked with that
// visitor.
//
// This is very similar to how go/ast.Walk works.
func walkParam(p param, v paramVisitor) {
	v = v.Visit(p)
	if v == nil {
		return
	}

	switch par := p.(type) {
	case paramSingle:
	case paramGroupedSlice:
	case paramObject:
		for _, f := range par.Fields {
			walkParam(f.Param, v)
		}
	case paramList:
		for _, p := range par.Params {
			walkParam(p, v)
		}
	default:
		panic(fmt.Sprintf(
			"It looks like you have found a bug in dig. "+
				"Please file an issue at https://github.com/uber-go/dig/issues/ "+
				"and provide the following message: "+
				"received unknown param type %T", p))
	}
}
