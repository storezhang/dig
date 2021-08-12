package dig

import (
	`reflect`

	`github.com/storezhang/dig/internal/dot`
)

type containerStore interface {
	containerWriter

	knownTypes() []reflect.Type
	getValue(name string, t reflect.Type) (v reflect.Value, ok bool)
	getValueGroup(name string, t reflect.Type) []reflect.Value
	getValueProviders(name string, t reflect.Type) []provider
	getGroupProviders(name string, t reflect.Type) []provider
	createGraph() *dot.Graph
	invoker() invokerFn
}
