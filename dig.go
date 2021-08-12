// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package dig

import (
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	`sync`
	"time"

	"github.com/storezhang/dig/internal/digreflect"
	"github.com/storezhang/dig/internal/dot"
)

const (
	_optionalTag         = "optional"
	_nameTag             = "name"
	_ignoreUnexportedTag = "ignore-unexported"
)

// Unique identification of an object in the graph.
type key struct {
	t reflect.Type

	// Only one of name or group will be set.
	name  string
	group string
}

// Option configures a Container. It's included for future functionality;
// currently, there are no concrete implementations.
type Option interface {
	applyOption(*Container)
}

type optionFunc func(*Container)

func (f optionFunc) applyOption(c *Container) { f(c) }

type provideOptions struct {
	Name  string
	Group string
	Info  *ProvideInfo
}

func (o *provideOptions) Validate() error {
	if len(o.Group) > 0 && len(o.Name) > 0 {
		return errf(
			"cannot use named values with value groups",
			"name:%q provided with group:%q", o.Name, o.Group)
	}

	// Names must be representable inside a backquoted string. The only
	// limitation for raw string literals as per
	// https://golang.org/ref/spec#raw_string_lit is that they cannot contain
	// backquotes.
	if strings.ContainsRune(o.Name, '`') {
		return errf("invalid dig.Name(%q): names cannot contain backquotes", o.Name)
	}
	if strings.ContainsRune(o.Group, '`') {
		return errf("invalid dig.Group(%q): group names cannot contain backquotes", o.Group)
	}
	return nil
}

// A ProvideOption modifies the default behavior of Provide.
type ProvideOption interface {
	applyProvideOption(*provideOptions)
}

type provideOptionFunc func(*provideOptions)

func (f provideOptionFunc) applyProvideOption(opts *provideOptions) { f(opts) }

// Name is a ProvideOption that specifies that all values produced by a
// constructor should have the given name. See also the package documentation
// about Named Values.
//
// Given,
//
//   func NewReadOnlyConnection(...) (*Connection, error)
//   func NewReadWriteConnection(...) (*Connection, error)
//
// The following will provide two connections to the container: one under the
// name "ro" and the other under the name "rw".
//
//   c.Provide(NewReadOnlyConnection, dig.Name("ro"))
//   c.Provide(NewReadWriteConnection, dig.Name("rw"))
//
// This option cannot be provided for constructors which produce result
// objects.
func Name(name string) ProvideOption {
	return provideOptionFunc(func(opts *provideOptions) {
		opts.Name = name
	})
}

// Group is a ProvideOption that specifies that all values produced by a
// constructor should be added to the specified group. See also the package
// documentation about Value Groups.
//
// This option cannot be provided for constructors which produce result
// objects.
func Group(group string) ProvideOption {
	return provideOptionFunc(func(opts *provideOptions) {
		opts.Group = group
	})
}

// ID is a unique integer representing the constructor node in the dependency graph.
type ID int

// ProvideInfo provides information about the constructor's inputs and outputs
// types as strings, as well as the ID of the constructor supplied to the Container.
// It contains ID for the constructor, as well as slices of Input and Output types,
// which are Stringers that report the types of the parameters and results respectively.
type ProvideInfo struct {
	ID      ID
	Inputs  []*Input
	Outputs []*Output
}

// Input contains information on an input parameter of the constructor.
type Input struct {
	t           reflect.Type
	optional    bool
	name, group string
}

func (i *Input) String() string {
	toks := make([]string, 0, 3)
	t := i.t.String()
	if i.optional {
		toks = append(toks, "optional")
	}
	if i.name != "" {
		toks = append(toks, fmt.Sprintf("name = %q", i.name))
	}
	if i.group != "" {
		toks = append(toks, fmt.Sprintf("group = %q", i.group))
	}

	if len(toks) == 0 {
		return t
	}
	return fmt.Sprintf("%v[%v]", t, strings.Join(toks, ", "))
}

// Output contains information on an output produced by the constructor.
type Output struct {
	t           reflect.Type
	name, group string
}

func (o *Output) String() string {
	toks := make([]string, 0, 2)
	t := o.t.String()
	if o.name != "" {
		toks = append(toks, fmt.Sprintf("name = %q", o.name))
	}
	if o.group != "" {
		toks = append(toks, fmt.Sprintf("group = %q", o.group))
	}

	if len(toks) == 0 {
		return t
	}
	return fmt.Sprintf("%v[%v]", t, strings.Join(toks, ", "))
}

// FillProvideInfo is a ProvideOption that writes info on what Dig was able to get out
// out of the provided constructor into the provided ProvideInfo.
func FillProvideInfo(info *ProvideInfo) ProvideOption {
	return provideOptionFunc(func(opts *provideOptions) {
		opts.Info = info
	})
}

// An InvokeOption modifies the default behavior of Invoke. It's included for
// future functionality; currently, there are no concrete implementations.
type InvokeOption interface {
	unimplemented()
}

// provider encapsulates a user-provided constructor.
type provider interface {
	ID() dot.CtorID
	Location() *digreflect.Func
	ParamList() paramList
	ResultList() resultList
	Call(containerStore) error
}

// New constructs a Container.
func New(opts ...Option) *Container {
	c := &Container{
		providers: make(map[key][]*node),
		values:    sync.Map{},
		groups:    make(map[key][]reflect.Value),
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
		invokerFn: defaultInvoker,
	}

	for _, opt := range opts {
		opt.applyOption(c)
	}
	return c
}

// DeferAcyclicVerification is an Option to override the default behavior
// of container.Provide, deferring the dependency graph validation to no longer
// run after each call to container.Provide. The container will instead verify
// the graph on first `Invoke`.
//
// Applications adding providers to a container in a tight loop may experience
// performance improvements by initializing the container with this option.
func DeferAcyclicVerification() Option {
	return optionFunc(func(c *Container) {
		c.deferAcyclicVerification = true
	})
}

// Changes the source of randomness for the container.
//
// This will help provide determinism during tests.
func setRand(r *rand.Rand) Option {
	return optionFunc(func(c *Container) {
		c.rand = r
	})
}

// DryRun is an Option which, when set to true, disables invocation of functions supplied to
// Provide and Invoke. Use this to build no-op containers.
func DryRun(dry bool) Option {
	return optionFunc(func(c *Container) {
		if dry {
			c.invokerFn = dryInvoker
		} else {
			c.invokerFn = defaultInvoker
		}
	})
}

// invokerFn specifies how the container calls user-supplied functions.
type invokerFn func(fn reflect.Value, args []reflect.Value) (results []reflect.Value)

func defaultInvoker(fn reflect.Value, args []reflect.Value) []reflect.Value {
	return fn.Call(args)
}

// Generates zero values for results without calling the supplied function.
func dryInvoker(fn reflect.Value, _ []reflect.Value) []reflect.Value {
	ft := fn.Type()
	results := make([]reflect.Value, ft.NumOut())
	for i := 0; i < ft.NumOut(); i++ {
		results[i] = reflect.Zero(fn.Type().Out(i))
	}

	return results
}

// Visits the results of a node and compiles a collection of all the keys
// produced by that node.
type connectionVisitor struct {
	c *Container
	n *node

	// If this points to a non-nil value, we've already encountered an error
	// and should stop traversing.
	err *error

	// Map of keys provided to path that provided this. The path is a string
	// documenting which positional return value or dig.Out attribute is
	// providing this particular key.
	//
	// For example, "[0].Foo" indicates that the value was provided by the Foo
	// attribute of the dig.Out returned as the first result of the
	// constructor.
	keyPaths map[key]string

	// We track the path to the current result here. For example, this will
	// be, ["[1]", "Foo", "Bar"] when we're visiting Bar in,
	//
	//   func() (io.Writer, struct {
	//     dig.Out
	//
	//     Foo struct {
	//       dig.Out
	//
	//       Bar io.Reader
	//     }
	//   })
	currentResultPath []string
}

func (cv connectionVisitor) AnnotateWithField(f resultObjectField) resultVisitor {
	cv.currentResultPath = append(cv.currentResultPath, f.FieldName)
	return cv
}

func (cv connectionVisitor) AnnotateWithPosition(i int) resultVisitor {
	cv.currentResultPath = append(cv.currentResultPath, fmt.Sprintf("[%d]", i))
	return cv
}

func (cv connectionVisitor) Visit(res result) resultVisitor {
	// Already failed. Stop looking.
	if *cv.err != nil {
		return nil
	}

	path := strings.Join(cv.currentResultPath, ".")

	switch r := res.(type) {
	case resultSingle:
		k := key{name: r.Name, t: r.Type}

		if conflict, ok := cv.keyPaths[k]; ok {
			*cv.err = errf(
				"cannot provide %v from %v", k, path,
				"already provided by %v", conflict,
			)
			return nil
		}

		if ps := cv.c.providers[k]; len(ps) > 0 {
			cons := make([]string, len(ps))
			for i, p := range ps {
				cons[i] = fmt.Sprint(p.Location())
			}

			*cv.err = errf(
				"cannot provide %v from %v", k, path,
				"already provided by %v", strings.Join(cons, "; "),
			)
			return nil
		}

		cv.keyPaths[k] = path

	case resultGrouped:
		// we don't really care about the path for this since conflicts are
		// okay for group results. We'll track it for the sake of having a
		// value there.
		k := key{group: r.Group, t: r.Type}
		cv.keyPaths[k] = path
	}

	return cv
}

// node is a node in the dependency graph. Each node maps to a single
// constructor provided by the user.
//
// Nodes can produce zero or more values that they store into the container.
// For the Provide path, we verify that nodes produce at least one value,
// otherwise the function will never be called.

type nodeOptions struct {
	// If specified, all values produced by this node have the provided name
	// or belong to the specified value group
	ResultName  string
	ResultGroup string
}

// Call calls this node's constructor if it hasn't already been called and
// injects any values produced by it into the provided container.
func (n *node) Call(c containerStore) error {
	if n.called {
		return nil
	}

	if err := shallowCheckDependencies(c, n.paramList); err != nil {
		return errMissingDependencies{
			Func:   n.location,
			Reason: err,
		}
	}

	args, err := n.paramList.BuildList(c)
	if err != nil {
		return errArgumentsFailed{
			Func:   n.location,
			Reason: err,
		}
	}

	receiver := newStagingContainerWriter()
	results := c.invoker()(reflect.ValueOf(n.ctor), args)
	if err := n.resultList.ExtractList(receiver, results); err != nil {
		return errConstructorFailed{Func: n.location, Reason: err}
	}
	receiver.Commit(c)
	n.called = true

	return nil
}

// Checks if a field of an In struct is optional.
func isFieldOptional(f reflect.StructField) (bool, error) {
	tag := f.Tag.Get(_optionalTag)
	if tag == "" {
		return false, nil
	}

	optional, err := strconv.ParseBool(tag)
	if err != nil {
		err = errf(
			"invalid value %q for %q tag on field %v",
			tag, _optionalTag, f.Name, err)
	}

	return optional, err
}

// Checks if ignoring unexported files in an In struct is allowed.
// The struct field MUST be an _inType.
func isIgnoreUnexportedSet(f reflect.StructField) (bool, error) {
	tag := f.Tag.Get(_ignoreUnexportedTag)
	if tag == "" {
		return false, nil
	}

	allowed, err := strconv.ParseBool(tag)
	if err != nil {
		err = errf(
			"invalid value %q for %q tag on field %v",
			tag, _ignoreUnexportedTag, f.Name, err)
	}

	return allowed, err
}

// Checks that all direct dependencies of the provided param are present in
// the container. Returns an error if not.
func shallowCheckDependencies(c containerStore, p param) error {
	var err errMissingTypes
	var addMissingNodes []*dot.Param
	walkParam(p, paramVisitorFunc(func(p param) bool {
		ps, ok := p.(paramSingle)
		if !ok {
			return true
		}

		if ns := c.getValueProviders(ps.Name, ps.Type); len(ns) == 0 && !ps.Optional {
			err = append(err, newErrMissingTypes(c, key{name: ps.Name, t: ps.Type})...)
			addMissingNodes = append(addMissingNodes, ps.DotParam()...)
		}

		return true
	}))

	if len(err) > 0 {
		return err
	}
	return nil
}

// stagingContainerWriter is a containerWriter that records the changes that
// would be made to a containerWriter and defers them until Commit is called.
type stagingContainerWriter struct {
	values map[key]reflect.Value
	groups map[key][]reflect.Value
}

var _ containerWriter = (*stagingContainerWriter)(nil)

func newStagingContainerWriter() *stagingContainerWriter {
	return &stagingContainerWriter{
		values: make(map[key]reflect.Value),
		groups: make(map[key][]reflect.Value),
	}
}

func (sr *stagingContainerWriter) setValue(name string, t reflect.Type, v reflect.Value) {
	sr.values[key{t: t, name: name}] = v
}

func (sr *stagingContainerWriter) submitGroupedValue(group string, t reflect.Type, v reflect.Value) {
	k := key{t: t, group: group}
	sr.groups[k] = append(sr.groups[k], v)
}

// Commit commits the received results to the provided containerWriter.
func (sr *stagingContainerWriter) Commit(cw containerWriter) {
	for k, v := range sr.values {
		cw.setValue(k.name, k.t, v)
	}

	for k, vs := range sr.groups {
		for _, v := range vs {
			cw.submitGroupedValue(k.group, k.t, v)
		}
	}
}

type byTypeName []reflect.Type

func (bs byTypeName) Len() int {
	return len(bs)
}

func (bs byTypeName) Less(i int, j int) bool {
	return fmt.Sprint(bs[i]) < fmt.Sprint(bs[j])
}

func (bs byTypeName) Swap(i int, j int) {
	bs[i], bs[j] = bs[j], bs[i]
}

func shuffledCopy(rand *rand.Rand, items []reflect.Value) []reflect.Value {
	newItems := make([]reflect.Value, len(items))
	for i, j := range rand.Perm(len(items)) {
		newItems[i] = items[j]
	}
	return newItems
}
