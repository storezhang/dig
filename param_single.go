package dig

import (
	`fmt`
	`reflect`
	`strings`

	`github.com/storezhang/dig/internal/dot`
)

type paramSingle struct {
	Name     string
	Optional bool
	Type     reflect.Type
}

func (ps paramSingle) DotParam() []*dot.Param {
	return []*dot.Param{
		{
			Node: &dot.Node{
				Type: ps.Type,
				Name: ps.Name,
			},
			Optional: ps.Optional,
		},
	}
}

func (ps paramSingle) Build(c containerStore) (reflect.Value, error) {
	if v, ok := c.getValue(ps.Name, ps.Type); ok {
		return v, nil
	}

	providers := c.getValueProviders(ps.Name, ps.Type)
	if len(providers) == 0 {
		if ps.Optional {
			return reflect.Zero(ps.Type), nil
		}
		return _noValue, newErrMissingTypes(c, key{name: ps.Name, t: ps.Type})
	}

	for _, n := range providers {
		err := n.Call(c)
		if err == nil {
			continue
		}

		// If we're missing dependencies but the parameter itself is optional,
		// we can just move on.
		if _, ok := err.(errMissingDependencies); ok && ps.Optional {
			return reflect.Zero(ps.Type), nil
		}

		return _noValue, errParamSingleFailed{
			CtorID: n.ID(),
			Key:    key{t: ps.Type, name: ps.Name},
			Reason: err,
		}
	}

	// If we get here, it's impossible for the value to be absent from the
	// container.
	v, _ := c.getValue(ps.Name, ps.Type)
	return v, nil
}

func (sp paramSingle) String() string {
	// tally.Scope[optional] means optional
	// tally.Scope[optional, name="foo"] means named optional

	var opts []string
	if sp.Optional {
		opts = append(opts, "optional")
	}
	if sp.Name != "" {
		opts = append(opts, fmt.Sprintf("name=%q", sp.Name))
	}

	if len(opts) == 0 {
		return fmt.Sprint(sp.Type)
	}

	return fmt.Sprintf("%v[%v]", sp.Type, strings.Join(opts, ", "))
}
