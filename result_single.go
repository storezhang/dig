package dig

import (
	`reflect`

	`github.com/storezhang/dig/internal/dot`
)

var _ result = resultSingle{}

type resultSingle struct {
	Name string
	Type reflect.Type
}

func (rs resultSingle) DotResult() []*dot.Result {
	return []*dot.Result{
		{
			Node: &dot.Node{
				Type: rs.Type,
				Name: rs.Name,
			},
		},
	}
}

func (rs resultSingle) Extract(cw containerWriter, v reflect.Value) {
	cw.setValue(rs.Name, rs.Type, v)
}
