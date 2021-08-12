package dig

import (
	`errors`
	`math/rand`
	`reflect`
	`sort`
	`sync`

	`github.com/storezhang/dig/internal/digreflect`
)

type Container struct {
	providers                map[key][]*node
	nodes                    []*node
	values                   sync.Map
	groups                   map[key][]reflect.Value
	rand                     *rand.Rand
	isVerifiedAcyclic        bool
	deferAcyclicVerification bool
	invokerFn                invokerFn
}

func (c *Container) knownTypes() []reflect.Type {
	typeSet := make(map[reflect.Type]struct{}, len(c.providers))
	for k := range c.providers {
		typeSet[k.t] = struct{}{}
	}

	types := make([]reflect.Type, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	sort.Sort(byTypeName(types))
	return types
}

func (c *Container) getValue(name string, reflectType reflect.Type) (value reflect.Value, ok bool) {
	var cachedValue interface{}
	if cachedValue, ok = c.values.Load(key{name: name, t: reflectType}); ok {
		value = cachedValue.(reflect.Value)
	}

	return
}

func (c *Container) setValue(name string, t reflect.Type, v reflect.Value) {
	c.values.Store(key{name: name, t: t}, v)
}

func (c *Container) getValueGroup(name string, t reflect.Type) []reflect.Value {
	items := c.groups[key{group: name, t: t}]

	return shuffledCopy(c.rand, items)
}

func (c *Container) submitGroupedValue(name string, t reflect.Type, v reflect.Value) {
	k := key{group: name, t: t}
	c.groups[k] = append(c.groups[k], v)
}

func (c *Container) getValueProviders(name string, t reflect.Type) []provider {
	return c.getProviders(key{name: name, t: t})
}

func (c *Container) getGroupProviders(name string, t reflect.Type) []provider {
	return c.getProviders(key{group: name, t: t})
}

func (c *Container) getProviders(k key) []provider {
	nodes := c.providers[k]
	providers := make([]provider, len(nodes))
	for i, n := range nodes {
		providers[i] = n
	}
	return providers
}

// invokerFn return a function to run when calling function provided to Provide or Invoke. Used for
// running container in dry mode.
func (c *Container) invoker() invokerFn {
	return c.invokerFn
}

// Provide teaches the container how to build values of one or more types and
// expresses their dependencies.
//
// The first argument of Provide is a function that accepts zero or more
// parameters and returns one or more results. The function may optionally
// return an error to indicate that it failed to build the value. This
// function will be treated as the constructor for all the types it returns.
// This function will be called AT MOST ONCE when a type produced by it, or a
// type that consumes this function's output, is requested via Invoke. If the
// same types are requested multiple times, the previously produced value will
// be reused.
//
// In addition to accepting constructors that accept dependencies as separate
// arguments and produce results as separate return values, Provide also
// accepts constructors that specify dependencies as dig.In structs and/or
// specify results as dig.Out structs.
func (c *Container) Provide(constructor interface{}, opts ...ProvideOption) error {
	ctype := reflect.TypeOf(constructor)
	if ctype == nil {
		return errors.New("can't provide an untyped nil")
	}
	if ctype.Kind() != reflect.Func {
		return errf("must provide constructor function, got %v (type %v)", constructor, ctype)
	}

	var options provideOptions
	for _, o := range opts {
		o.applyProvideOption(&options)
	}
	if err := options.Validate(); err != nil {
		return err
	}

	if err := c.provide(constructor, options); err != nil {
		return errProvide{
			Func:   digreflect.InspectFunc(constructor),
			Reason: err,
		}
	}
	return nil
}

func (c *Container) Invoke(function interface{}, _ ...InvokeOption) error {
	ftype := reflect.TypeOf(function)
	if ftype == nil {
		return errors.New("can't invoke an untyped nil")
	}
	if ftype.Kind() != reflect.Func {
		return errf("can't invoke non-function %v (type %v)", function, ftype)
	}

	pl, err := newParamList(ftype)
	if err != nil {
		return err
	}

	if err := shallowCheckDependencies(c, pl); err != nil {
		return errMissingDependencies{
			Func:   digreflect.InspectFunc(function),
			Reason: err,
		}
	}

	if !c.isVerifiedAcyclic {
		if err := c.verifyAcyclic(); err != nil {
			return err
		}
	}

	args, err := pl.BuildList(c)
	if err != nil {
		return errArgumentsFailed{
			Func:   digreflect.InspectFunc(function),
			Reason: err,
		}
	}
	returned := c.invokerFn(reflect.ValueOf(function), args)
	if len(returned) == 0 {
		return nil
	}
	if last := returned[len(returned)-1]; isError(last.Type()) {
		if err, _ := last.Interface().(error); err != nil {
			return err
		}
	}

	return nil
}

func (c *Container) verifyAcyclic() error {
	visited := make(map[key]struct{})
	for _, n := range c.nodes {
		if err := detectCycles(n, c, nil /* path */, visited); err != nil {
			return errf("cycle detected in dependency graph", err)
		}
	}

	c.isVerifiedAcyclic = true
	return nil
}

func (c *Container) provide(ctor interface{}, opts provideOptions) error {
	n, err := newNode(
		ctor,
		nodeOptions{
			ResultName:  opts.Name,
			ResultGroup: opts.Group,
		},
	)
	if err != nil {
		return err
	}

	keys, err := c.findAndValidateResults(n)
	if err != nil {
		return err
	}

	ctype := reflect.TypeOf(ctor)
	if len(keys) == 0 {
		return errf("%v must provide at least one non-error type", ctype)
	}

	for k := range keys {
		c.isVerifiedAcyclic = false
		oldProviders := c.providers[k]
		c.providers[k] = append(c.providers[k], n)

		if c.deferAcyclicVerification {
			continue
		}
		if err := verifyAcyclic(c, n, k); err != nil {
			c.providers[k] = oldProviders
			return err
		}
		c.isVerifiedAcyclic = true
	}
	c.nodes = append(c.nodes, n)

	// Record introspection info for caller if Info option is specified
	if info := opts.Info; info != nil {
		params := n.ParamList().DotParam()
		results := n.ResultList().DotResult()

		info.ID = (ID)(n.id)
		info.Inputs = make([]*Input, len(params))
		info.Outputs = make([]*Output, len(results))

		for i, param := range params {
			info.Inputs[i] = &Input{
				t:        param.Type,
				optional: param.Optional,
				name:     param.Name,
				group:    param.Group,
			}
		}

		for i, res := range results {
			info.Outputs[i] = &Output{
				t:     res.Type,
				name:  res.Name,
				group: res.Group,
			}
		}
	}
	return nil
}

// Builds a collection of all result types produced by this node.
func (c *Container) findAndValidateResults(n *node) (map[key]struct{}, error) {
	var err error
	keyPaths := make(map[key]string)
	walkResult(n.ResultList(), connectionVisitor{
		c:        c,
		n:        n,
		err:      &err,
		keyPaths: keyPaths,
	})

	if err != nil {
		return nil, err
	}

	keys := make(map[key]struct{}, len(keyPaths))
	for k := range keyPaths {
		keys[k] = struct{}{}
	}
	return keys, nil
}
