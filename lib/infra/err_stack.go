package infra

import (
	"fmt"
	"io"
	"path"
	"runtime"
	"strconv"
	"strings"
)

// References:
// https://github.com/pkg/errors/blob/master/stack.go

type Frame uintptr

func (frame Frame) pc() uintptr {
	return uintptr(frame) - 1
}

func (frame Frame) file() string {
	pc := frame.pc()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknownFile"
	}
	f, _ := fn.FileLine(pc)
	return f
}

func (frame Frame) line() int {
	pc := frame.pc()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return 0
	}
	_, l := fn.FileLine(pc)
	return l
}

func (frame Frame) name() string {
	pc := frame.pc()
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
func (frame Frame) Format(s fmt.State, verb rune) {
	switch verb {
	case 's':
		if s.Flag('+') {
			_, _ = io.WriteString(s, frame.name())
			_, _ = io.WriteString(s, "\n\t")
			_, _ = io.WriteString(s, frame.file())
		} else {
			_, _ = io.WriteString(s, path.Base(frame.file()))
		}
	case 'd':
		_, _ = io.WriteString(s, strconv.Itoa(frame.line()))
	case 'n':
		_, _ = io.WriteString(s, funcName(frame.name()))
	case 'v':
		frame.Format(s, 's')
		_, _ = io.WriteString(s, ":")
		frame.Format(s, 'd')
	}
}

// For fmt.Sprintf("%+v", frame).
// If json.Marshaler interface isn't implemented, the MarshalText method is used.
func (frame Frame) MarshalText() ([]byte, error) {
	name := frame.name()
	if name == "unknownFunc" {
		return []byte("unknownFrame"), nil
	}
	builder := strings.Builder{}
	_, _ = builder.WriteString(name)
	_, _ = builder.WriteString(" ")
	_, _ = builder.WriteString(frame.file())
	_, _ = builder.WriteString(":")
	_, _ = builder.WriteString(strconv.Itoa(frame.line()))
	return []byte(builder.String()), nil
}

func (frame Frame) MarshalJSON() ([]byte, error) {
	name := frame.name()
	if name == "unknownFunc" {
		return []byte("{\"frame\":\"unknownFrame\"}"), nil
	}
	builder := strings.Builder{}
	_, _ = builder.WriteString("{")
	_, _ = builder.WriteString("\"func\":\"")
	_, _ = builder.WriteString(name)
	_, _ = builder.WriteString("\",")
	_, _ = builder.WriteString("\"fileAndLine\":\"")
	_, _ = builder.WriteString(frame.file())
	_, _ = builder.WriteString(":")
	_, _ = builder.WriteString(strconv.Itoa(frame.line()))
	_, _ = builder.WriteString("\"}")
	return []byte(builder.String()), nil
}

func funcName(name string) string {
	i := strings.LastIndex(name, "/")
	name = name[i+1:]
	i = strings.Index(name, ".")
	return name[i+1:]
}
