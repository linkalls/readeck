// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package contentscripts provides a JavaScript engine that runs
// builtin, or user defined, scripts during the extraction process.
package contentscripts

import (
	"cmp"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/url"
)

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "readeck/pkg/contentscript context value " + k.name
}

// Program is a wrapper around goja.Program, with a script name
type Program struct {
	*goja.Program
	Name     string
	Priority int
}

// Runtime contains a collection of content scripts
type Runtime struct {
	*goja.Runtime
	programs []*Program
	ctx      context.Context
}

type execFunc func() error

var registry = new(require.Registry)

// NewProgram wraps a script into an anonymous function call
// exposing the "exports" object and returns a Program instance.
func NewProgram(name string, r io.Reader) (*Program, error) {
	b := new(strings.Builder)
	b.WriteString("(function(exports) {\n")
	if _, err := io.Copy(b, r); err != nil {
		return nil, err
	}
	b.WriteString("\n})(exports)")

	prg, err := goja.Parse(name, b.String())
	if err != nil {
		return nil, err
	}

	p, err := goja.CompileAST(prg, true)
	if err != nil {
		return nil, err
	}
	return &Program{
		Name:     name,
		Program:  p,
		Priority: getPriority(prg),
	}, nil
}

// getPriority look into a (wrapped) program for the integer value
// of exports.priority
func getPriority(prg *ast.Program) (res int) {
	// The first element is the wrapping function call
	body := prg.Body[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression).Callee.(*ast.FunctionLiteral)

	// We loop in reversed order because we want the last
	// defined "exports.priority" and return fast
	for i := len(body.Body.List) - 1; i >= 0; i-- {
		item := body.Body.List[i]

		xps, ok := item.(*ast.ExpressionStatement)
		if !ok {
			continue
		}
		assign, ok := xps.Expression.(*ast.AssignExpression)
		if !ok {
			continue
		}
		if assign.Operator.String() != "=" {
			continue
		}
		right, ok := assign.Right.(*ast.NumberLiteral)
		if !ok {
			continue
		}
		left, ok := assign.Left.(*ast.DotExpression)
		if !ok || left.Identifier.Name != "priority" {
			continue
		}
		idt, ok := left.Left.(*ast.Identifier)
		if !ok || idt.Name != "exports" {
			continue
		}

		value, ok := right.Value.(int64)
		if ok {
			return int(value)
		}
	}

	return 0
}

// New creates a new ContentScript instance
func New(programs ...*Program) *Runtime {
	slices.SortStableFunc(programs, func(a, b *Program) int {
		if a.Priority != b.Priority {
			return cmp.Compare(a.Priority, b.Priority)
		}

		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	r := &Runtime{
		Runtime:  goja.New(),
		programs: programs,
		ctx:      context.Background(),
	}

	r.SetFieldNameMapper(goja.TagFieldNameMapper("js", true))
	r.startConsole()

	// Register utils
	registry.Enable(r.Runtime)
	url.Enable(r.Runtime)

	// Register global variables and functions
	registerExported(r)

	return r
}

func (vm *Runtime) getExports(name string) goja.Value {
	return vm.Get("exports").ToObject(vm.Runtime).Get(name)
}

// AddScript wraps a script into an anonymous function call exposing the
// "exports" object and adds it to the script list.
func (vm *Runtime) AddScript(name string, r io.Reader) error {
	p, err := NewProgram(name, r)
	if err != nil {
		return err
	}
	vm.programs = append(vm.programs, p)
	return nil
}

// RunProgram runs a Program instance in the VM and returns its result.
func (vm *Runtime) RunProgram(p *Program) (goja.Value, error) {
	time.AfterFunc(10*time.Second, func() {
		vm.Interrupt("timeout")
	})
	return vm.Runtime.RunProgram(p.Program)
}

func (vm *Runtime) exec(p *Program, fn execFunc) error {
	vm.Set("__name__", p.Name)
	vm.Set("exports", map[string]any{})

	defer func() {
		vm.GlobalObject().Delete("__name__")
		vm.GlobalObject().Delete("exports")
	}()

	_, err := vm.RunProgram(p)
	if err != nil {
		return err
	}

	// Check if the current script is active
	if ok, err := vm.isActive(); err != nil {
		return err
	} else if ok {
		if vm.getProcessMessage() != nil {
			vm.Set("requests", NewHTTPClient(vm, vm.getProcessMessage().Extractor.Client()))
			defer func() {
				vm.GlobalObject().Delete("requests")
			}()
		}
		return fn()
	}
	return nil
}

func (vm *Runtime) execEach(fn execFunc) error {
	errList := []error{}

	for _, p := range vm.programs {
		if err := vm.exec(p, fn); err != nil {
			// A failing script only logs its error
			// and let the other scripts carry on.
			errList = append(errList, err)
			vm.GetLogger().
				WithError(err).
				Error("content script error")
		}
	}

	return errors.Join(errList...)
}

func (vm *Runtime) isActive() (res bool, err error) {
	f := vm.getExports("isActive")
	if f == nil {
		return false, nil
	}

	var fn isActive
	if err = vm.ExportTo(f, &fn); err != nil {
		return false, err
	}
	return fn()
}

type isActive func() (bool, error)

// SetConfig runs every script and calls their respective
// "setConfig" exported function when it exists.
// The initial configuration is passed to each function as
// a pointer and can be modified in place.
func (vm *Runtime) SetConfig(cf *SiteConfig) error {
	return vm.execEach(func() error {
		f := vm.getExports("setConfig")
		if f == nil {
			return nil
		}
		var fn setConfig
		if err := vm.ExportTo(f, &fn); err != nil {
			return err
		}
		vm.GetLogger().
			WithField("function", "setConfig").
			Debug("content script")
		return fn(cf)
	})
}

type setConfig func(*SiteConfig) error

// ProcessMeta runs every script and calls their respective
// "processMeta" exported function when it exists.
func (vm *Runtime) ProcessMeta() error {
	return vm.execEach(func() error {
		f := vm.getExports("processMeta")
		if f == nil {
			return nil
		}
		var fn processMeta
		if err := vm.ExportTo(f, &fn); err != nil {
			return err
		}
		vm.GetLogger().
			WithField("function", "processMeta").
			Debug("content script")
		return fn()
	})
}

type processMeta func() error
