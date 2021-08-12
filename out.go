package dig

import (
	`reflect`
)

var (
	_outPtrType = reflect.TypeOf((*Out)(nil))
	_outType    = reflect.TypeOf(Out{})
)

// Out 表示输出
type Out struct {
	sentinel
}

func isOut(obj interface{}) bool {
	return isEmbed(obj, _outType)
}
