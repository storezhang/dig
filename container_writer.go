package dig

import (
	`reflect`
)

type containerWriter interface {
	setValue(name string, t reflect.Type, v reflect.Value)
	submitGroupedValue(name string, t reflect.Type, v reflect.Value)
}
