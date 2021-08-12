package dig

import (
	`reflect`

	`github.com/storezhang/dig/internal/digreflect`
	`github.com/storezhang/dig/internal/dot`
)

type node struct {
	ctor       interface{}
	ctype      reflect.Type
	location   *digreflect.Func
	id         dot.CtorID
	called     bool
	paramList  paramList
	resultList resultList
}

func newNode(ctor interface{}, opts nodeOptions) (*node, error) {
	cval := reflect.ValueOf(ctor)
	ctype := cval.Type()
	cptr := cval.Pointer()

	params, err := newParamList(ctype)
	if err != nil {
		return nil, err
	}

	results, err := newResultList(
		ctype,
		resultOptions{
			Name:  opts.ResultName,
			Group: opts.ResultGroup,
		},
	)
	if err != nil {
		return nil, err
	}

	return &node{
		ctor:       ctor,
		ctype:      ctype,
		location:   digreflect.InspectFunc(ctor),
		id:         dot.CtorID(cptr),
		paramList:  params,
		resultList: results,
	}, err
}

func (n *node) Location() *digreflect.Func {
	return n.location
}
func (n *node) ParamList() paramList {
	return n.paramList
}
func (n *node) ResultList() resultList {
	return n.resultList
}
func (n *node) ID() dot.CtorID {
	return n.id
}
