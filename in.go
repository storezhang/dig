package dig

import (
	`reflect`
)

var (
	_inPtrType = reflect.TypeOf((*In)(nil))
	_inType    = reflect.TypeOf(In{})
)

// In 表示输入
type In struct {
	sentinel
}

func isIn(obj interface{}) bool {
	return isEmbed(obj, _inType)
}
