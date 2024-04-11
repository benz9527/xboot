package infra

import (
	"errors"
	"fmt"
	"io"
	"path"
	"runtime"
	"strconv"
	"strings"

	"go.uber.org/multierr"
)

// References:
// https://github.com/pkg/errors/blob/master/stack.go

type frame uintptr

func (fr frame) pc() uintptr {
	return uintptr(fr) - 1
}

func (fr frame) file() string {
	pc := fr.pc()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknownFile"
	}
	f, _ := fn.FileLine(pc)
	return f
}

func (fr frame) line() int {
	pc := fr.pc()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return 0
	}
	_, l := fn.FileLine(pc)
	return l
}

func (fr frame) name() string {
	pc := fr.pc()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknownFunc"
	}
	return fn.Name()
}

// Format characters:
// %s - source file
// %d - source line
// %n - function name
// %v - verbose, equivalent to %s:%d
// %+s - full path, the root path is relative to the compile time GOPATH
// separated by \n\t (<function-name>\n\t<path>)
// %+v - equivalent to %+s:%d
func (fr frame) Format(state fmt.State, verb rune) {
	switch verb {
	case 's':
		if state.Flag('+') {
			_, _ = io.WriteString(state, fr.name())
			_, _ = io.WriteString(state, "\n\t")
			_, _ = io.WriteString(state, fr.file())
		} else {
			_, _ = io.WriteString(state, path.Base(fr.file()))
		}
	case 'd':
		_, _ = io.WriteString(state, strconv.Itoa(fr.line()))
	case 'n':
		_, _ = io.WriteString(state, funcName(fr.name()))
	case 'v':
		fr.Format(state, 's')
		_, _ = io.WriteString(state, ":")
		fr.Format(state, 'd')
	}
}

// For fmt.Sprintf("%+v", frame).
// If json.Marshaler interface isn't implemented, the MarshalText method is used.
func (fr frame) MarshalText() ([]byte, error) {
	name := fr.name()
	if name == "unknownFunc" {
		return []byte("unknownFrame"), nil
	}
	builder := strings.Builder{}
	_, _ = builder.WriteString(name)
	_, _ = builder.WriteString(" ")
	_, _ = builder.WriteString(fr.file())
	_, _ = builder.WriteString(":")
	_, _ = builder.WriteString(strconv.Itoa(fr.line()))
	return []byte(builder.String()), nil
}

func (fr frame) MarshalJSON() ([]byte, error) {
	name := fr.name()
	if name == "unknownFunc" {
		return []byte("{\"frame\":\"unknownFrame\"}"), nil
	}
	builder := strings.Builder{}
	_, _ = builder.WriteString("{")
	_, _ = builder.WriteString("\"func\":\"")
	_, _ = builder.WriteString(name)
	_, _ = builder.WriteString("\",")
	_, _ = builder.WriteString("\"fileAndLine\":\"")
	_, _ = builder.WriteString(fr.file())
	_, _ = builder.WriteString(":")
	_, _ = builder.WriteString(strconv.Itoa(fr.line()))
	_, _ = builder.WriteString("\"}")
	return []byte(builder.String()), nil
}

func funcName(name string) string {
	i := strings.LastIndex(name, "/")
	name = name[i+1:]
	i = strings.Index(name, ".")
	return name[i+1:]
}

type stackTrace []frame

func (st stackTrace) Format(state fmt.State, verb rune) {
	switch verb {
	case 'v':
		if state.Flag('#') {
			_, _ = io.WriteString(state, "infra.stackTrace(")
			if len(st) > 0 {
				_, _ = io.WriteString(state, "[")
				for i, frame := range st {
					if i > 0 {
						_, _ = io.WriteString(state, ",")
					}
					frame.Format(state, verb)
				}
				_, _ = io.WriteString(state, "]")
			} else {
				_, _ = io.WriteString(state, "nil")
			}
			_, _ = io.WriteString(state, ")")
		} else if state.Flag('+') {
			for i, frame := range st {
				if i > 0 {
					_, _ = io.WriteString(state, "\n")
				}
				frame.Format(state, verb)
			}
			if len(st) <= 0 {
				_, _ = io.WriteString(state, "nil")
			}
		} else {
			st.formatSlice(state, verb)
		}
	case 's':
		st.formatSlice(state, verb)
	}
}

func (st stackTrace) formatSlice(state fmt.State, verb rune) {
	if !state.Flag('+') {
		_, _ = io.WriteString(state, "[")
		for i, frame := range st {
			if i > 0 {
				_, _ = io.WriteString(state, " ")
			}
			frame.Format(state, verb)
		}
		_, _ = io.WriteString(state, "]")
	} else {
		for i, frame := range st {
			if i > 0 {
				_, _ = io.WriteString(state, "\n")
			}
			frame.Format(state, verb)
		}
	}
}

func (st stackTrace) MarshalText() ([]byte, error) {
	builder := strings.Builder{}
	for i, frame := range st {
		if i > 0 {
			_, _ = builder.WriteString("\n")
		}
		_bytes, _ := frame.MarshalText()
		_, _ = builder.Write(_bytes)
	}
	return []byte(builder.String()), nil
}

func (st stackTrace) MarshalJSON() ([]byte, error) {
	builder := strings.Builder{}
	_, _ = builder.WriteString("[")
	for i, frame := range st {
		if i > 0 {
			_, _ = builder.WriteString(",")
		}
		_bytes, _ := frame.MarshalJSON()
		_, _ = builder.Write(_bytes)
	}
	_, _ = builder.WriteString("]")
	return []byte(builder.String()), nil
}

type stack []uintptr

func (stack stack) StackTrace() stackTrace {
	st := make(stackTrace, len(stack))
	for i := 0; i < len(st); i++ {
		st[i] = frame(stack[i])
	}
	return st
}

func (stack stack) Format(state fmt.State, verb rune) {
	switch verb {
	case 'v':
		if state.Flag('#') {
			_, _ = io.WriteString(state, "infra.stack(")
			stack.StackTrace().Format(state, verb)
			_, _ = io.WriteString(state, ")")
		} else if state.Flag('+') {
			_, _ = io.WriteString(state, "stack:\n")
			st := stack.StackTrace()
			_, _ = fmt.Fprintf(state, "%+v", st)
		} else {
			stack.StackTrace().Format(state, 'v')
		}
	}
}

func (st stack) MarshalJSON() ([]byte, error) {
	builder := strings.Builder{}
	_bytes, _ := st.StackTrace().MarshalJSON()
	_, _ = builder.Write(_bytes)
	return []byte(builder.String()), nil
}

func getCallers(depth int8) stack {
	pcs := make([]uintptr, depth)
	n := runtime.Callers(3, pcs[:])
	var st stack = pcs[0:n]
	return st
}

type errorStack struct {
	err   error
	stack *stack
	upper *errorStack
}

// For errors.Is(err, target error) and errors.As(err error, target any).
func (es *errorStack) Unwrap() []error {
	_errors := make([]error, 0, 8)
	for err := es.err; es != nil; es = es.upper {
		switch x := err.(type) {
		case interface{ Unwrap() []error }:
			_errors = append(_errors, x.Unwrap()...)
		default:
			_errors = append(_errors, err)
		}
	}
	return _errors
}

func (es *errorStack) Error() string {
	builder := strings.Builder{}
	for i, iter := 0, es; iter != nil; i, iter = i+1, iter.upper {
		if iter.err != nil {
			if i > 0 {
				_, _ = builder.WriteString("; ")
			}
			_, _ = builder.WriteString(iter.err.Error())
		}
	}
	res := builder.String()
	if res == "" {
		res = "[{}]"
	}
	return res
}

func (es *errorStack) Format(state fmt.State, verb rune) {
	switch verb {
	case 'v':
		if state.Flag('#') {
			_, _ = io.WriteString(state, "infra.errorStacks([")
			for i, iter := 0, es; iter != nil; i, iter = i+1, iter.upper {
				if i > 0 {
					_, _ = io.WriteString(state, ",")
				}
				_, _ = io.WriteString(state, "infra.errorStack({")
				_, _ = io.WriteString(state, "error(")
				if iter.err != nil {
					_, _ = io.WriteString(state, iter.err.Error())
				} else {
					_, _ = io.WriteString(state, "nil")
				}
				_, _ = io.WriteString(state, "); ")
				if iter.stack != nil {
					iter.stack.Format(state, verb)
				} else {
					_, _ = io.WriteString(state, "nilStack")
				}
				_, _ = io.WriteString(state, "})")
			}
			_, _ = io.WriteString(state, "])")
		} else if state.Flag('+') {
			for i, iter := 0, es; iter != nil; i, iter = i+1, iter.upper {
				if i > 0 {
					_, _ = io.WriteString(state, "\n")
				}
				_, _ = io.WriteString(state, "error messages:\n\t")
				if iter.err != nil {
					_, _ = io.WriteString(state, iter.err.Error())
				} else {
					_, _ = io.WriteString(state, "nil")
				}
				_, _ = io.WriteString(state, "\n")
				if iter.stack != nil {
					iter.stack.Format(state, verb)
				} else {
					_, _ = io.WriteString(state, "no stack!")
				}
			}
		} else {
			_, _ = io.WriteString(state, es.Error())
		}
	case 's':
		_, _ = io.WriteString(state, es.Error())
	}
}

func (es *errorStack) MarshalJSON() ([]byte, error) {
	builder := strings.Builder{}
	_, _ = builder.WriteString("[")
	for i, iter := 0, es; iter != nil; i, iter = i+1, iter.upper {
		if i > 0 {
			_, _ = builder.WriteString(",")
		}
		_, _ = builder.WriteString("{")
		if iter.err != nil {
			_, _ = builder.WriteString("\"error\":\"")
			_, _ = builder.WriteString(iter.err.Error())
			_, _ = builder.WriteString("\"")
		}
		if iter.stack != nil {
			_, _ = builder.WriteString(",")
			_, _ = builder.WriteString("\"errorStack\":")
			_bytes, _ := iter.stack.MarshalJSON()
			_, _ = builder.Write(_bytes)
		}
		_, _ = builder.WriteString("}")
	}
	_, _ = builder.WriteString("]")
	return []byte(builder.String()), nil
}

func NewErrorStack(errMsg string) error {
	errMsg = strings.TrimSpace(errMsg)
	if len(errMsg) <= 0 {
		return nil
	}
	s := getCallers(32)
	return &errorStack{
		err:   errors.New(errMsg),
		stack: &s,
		upper: nil,
	}
}

func WrapErrorStack(err error) error {
	if err == nil {
		return nil
	}
	if es, ok := err.(*errorStack); ok && es.err != nil && es.stack != nil {
		return es
	}
	s := getCallers(32)
	return &errorStack{
		err:   err,
		stack: &s,
		upper: nil,
	}
}

func AppendErrorStack(es error, errors ...error) error {
	if len(errors) <= 0 {
		return es
	}

	var merr error
	for _, e := range errors {
		if e != nil {
			merr = multierr.Append(merr, e)
		}
	}
	if es == nil {
		s := getCallers(32)
		return &errorStack{
			err:   merr,
			stack: &s,
			upper: nil,
		}
	}
	if _es, ok := es.(*errorStack); ok && _es.err != nil && _es.stack != nil {

		s := getCallers(32)
		return &errorStack{
			err:   merr,
			stack: &s,
			upper: _es,
		}
	}
	if estr := es.Error(); len(estr) > 0 && estr != "[{}]" {
		merr = multierr.Append(es, merr)
	}
	s := getCallers(32)
	return &errorStack{
		err:   merr,
		stack: &s,
		upper: nil,
	}
}

func WrapErrorStackWithMessage(es error, errMsg string) error {
	if len(errMsg) <= 0 {
		return es
	}
	err := fmt.Errorf("%s", errMsg)
	if es == nil {
		s := getCallers(32)
		return &errorStack{
			err:   err,
			stack: &s,
			upper: nil,
		}
	}
	if _es, ok := es.(*errorStack); ok && _es.err != nil && _es.stack != nil {
		s := getCallers(32)
		return &errorStack{
			err:   err,
			stack: &s,
			upper: _es,
		}
	}
	if estr := es.Error(); len(estr) > 0 && estr != "[{}]" {
		err = multierr.Append(es, err)
	}
	s := getCallers(32)
	return &errorStack{
		err:   err,
		stack: &s,
		upper: nil,
	}
}
