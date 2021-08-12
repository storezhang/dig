package dig

import (
	`container/list`
	`reflect`
)

func isEmbed(obj interface{}, reflectType reflect.Type) (is bool) {
	if nil == obj {
		return
	}

	realType, ok := obj.(reflect.Type)
	if !ok {
		realType = reflect.TypeOf(obj)
	}

	types := list.New()
	types.PushBack(realType)
	for types.Len() > 0 {
		frontType := types.Remove(types.Front()).(reflect.Type)
		if reflectType == frontType {
			is = true
		}
		if is {
			return
		}

		if reflect.Struct != frontType.Kind() {
			continue
		}

		for i := 0; i < frontType.NumField(); i++ {
			field := frontType.Field(i)
			if field.Anonymous {
				types.PushBack(field.Type)
			}
		}
	}

	return
}
