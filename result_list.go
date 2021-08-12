package dig

import (
	`reflect`

	`github.com/storezhang/dig/internal/dot`
)

var _ result = resultList{}

type resultList struct {
	ctype         reflect.Type
	Results       []result
	resultIndexes []int
}

func newResultList(ctype reflect.Type, opts resultOptions) (resultList, error) {
	rl := resultList{
		ctype:         ctype,
		Results:       make([]result, 0, ctype.NumOut()),
		resultIndexes: make([]int, ctype.NumOut()),
	}

	resultIdx := 0
	for i := 0; i < ctype.NumOut(); i++ {
		t := ctype.Out(i)
		if isError(t) {
			rl.resultIndexes[i] = -1
			continue
		}

		r, err := newResult(t, opts)
		if err != nil {
			return rl, errf("bad result %d", i+1, err)
		}

		rl.Results = append(rl.Results, r)
		rl.resultIndexes[i] = resultIdx
		resultIdx++
	}

	return rl, nil
}

func (rl resultList) DotResult() []*dot.Result {
	var types []*dot.Result
	for _, result := range rl.Results {
		types = append(types, result.DotResult()...)
	}
	return types
}

func (resultList) Extract(containerWriter, reflect.Value) {
	panic("It looks like you have found a bug in dig. " +
		"Please file an issue at https://github.com/uber-go/dig/issues/ " +
		"and provide the following message: " +
		"resultList.Extract() must never be called")
}

func (rl resultList) ExtractList(cw containerWriter, values []reflect.Value) error {
	for i, v := range values {
		if resultIdx := rl.resultIndexes[i]; resultIdx >= 0 {
			rl.Results[resultIdx].Extract(cw, v)
			continue
		}

		if err, _ := v.Interface().(error); err != nil {
			return err
		}
	}

	return nil
}
