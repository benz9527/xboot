package infra

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

var initPC = caller()

func caller() frame {
	var PCs [3]uintptr
	n := runtime.Callers(2, PCs[:])
	frames := runtime.CallersFrames(PCs[:n])
	fr, _ := frames.Next()
	return frame(fr.PC)
}

func TestFrameFormat(t *testing.T) {
	testcases := []struct {
		frame
		format       string
		expectedExpr string
	}{
		{
			initPC,
			"%s",
			"^err_stack_test.go$",
		},
		{
			initPC,
			"%+s",
			"^github.com/benz9527/xboot/lib/infra.init\\n\\t.*xboot/lib/infra/err_stack_test.go$",
		},
		{
			initPC,
			"%n",
			"^init$",
		},
		{
			initPC,
			"%d",
			"\\d+",
		},
		{
			initPC,
			"%v",
			"^err_stack_test.go:\\d+$",
		},
		{
			initPC,
			"%+v",
			"^github.com/benz9527/xboot/lib/infra.init\\n\\t.*xboot/lib/infra/err_stack_test.go:\\d+$",
		},
		{
			frame(0),
			"%s",
			"unknownFile",
		},
		{
			frame(0),
			"%n",
			"unknownFunc",
		},
		{
			frame(0),
			"%d",
			"0",
		},
	}

	for _, tc := range testcases {
		expr := regexp.MustCompile(tc.expectedExpr)
		require.NotNil(t, expr)
		frameRes := fmt.Sprintf(tc.format, tc.frame)
		require.True(t, expr.MatchString(frameRes))
	}
}

func TestFrameMarshalText(t *testing.T) {
	testcases := []struct {
		frame
		expectedExpr string
	}{
		{
			initPC,
			"^github.com/benz9527/xboot/lib/infra.init .*xboot/lib/infra/err_stack_test.go:\\d+$",
		},
		{
			frame(0),
			"unknownFrame",
		},
	}
	for _, tc := range testcases {
		_bytes, err := tc.frame.MarshalText()
		require.NoError(t, err)
		require.Greater(t, len(_bytes), 0)
		expr := regexp.MustCompile(tc.expectedExpr)
		require.True(t, expr.MatchString(string(_bytes)))
	}
}

func TestFrameMarshalJSON(t *testing.T) {
	testcases := []struct {
		frame
		expectedExpr string
	}{
		{
			initPC,
			"^{\"func\":\"github\\.com/benz9527/xboot/lib/infra\\.init\",\"fileAndLine\":\".*xboot/lib/infra/err_stack_test\\.go:\\d+\"}$",
		},
		{
			frame(0),
			"{\"frame\":\"unknownFrame\"}",
		},
	}
	for _, tc := range testcases {
		_bytes, err := json.Marshal(tc.frame)
		require.NoError(t, err)
		require.Greater(t, len(_bytes), 0)
		expr := regexp.MustCompile(tc.expectedExpr)
		require.NotNil(t, expr)
		require.True(t, expr.MatchString(string(_bytes)))
	}
}

func TestStackTraceFormat(t *testing.T) {
	testcases := []struct {
		stackTrace
		format       string
		expectedExpr string
	}{
		{
			[]frame{},
			"%s",
			"\\[\\]",
		},
		{
			nil,
			"%s",
			"\\[\\]",
		},
		{
			nil,
			"%+v",
			"nil",
		},
		{
			nil,
			"%#v",
			"infra.stackTrace\\(nil\\)",
		},
		{
			testStackTrace()[:4],
			"%v",
			"\\[(.*\\.(go|s):\\d+ ?)+\\]",
		},
		{
			testStackTrace()[:4],
			"%#v",
			"infra.stackTrace\\(\\[(.*\\.(go|s):\\d+,?)+\\]\\)",
		},
		{
			testStackTrace()[:2],
			"%+v",
			`(github\.com/benz9527/xboot/lib/infra\..*\n\t.*\.go:\d+\n?){2}`,
		},
		{
			testStackTrace()[:2],
			"%+s",
			`(github\.com/benz9527/xboot/lib/infra\..*\n\t.*\.go\n?){2}`,
		},
	}
	for _, tc := range testcases {
		expr := regexp.MustCompile(tc.expectedExpr)
		require.NotNil(t, expr)
		st := fmt.Sprintf(tc.format, tc.stackTrace)
		require.True(t, expr.MatchString(st))
	}
}

func testStackTrace() stackTrace {
	const depth = 8
	var pcs [depth]uintptr
	n := runtime.Callers(1, pcs[:])
	var st stack = pcs[0:n]
	return st.StackTrace()
}

func TestStackTraceMarshalText(t *testing.T) {
	testcases := []struct {
		stackTrace
		expectedExpr string
	}{
		{
			[]frame{},
			"",
		},
		{
			nil,
			"",
		},
		{
			testStackTrace()[:2],
			`(github\.com/benz9527/xboot/lib/infra\..* .*\.go:\d+\n?){2}`,
		},
	}
	for _, tc := range testcases {
		expr := regexp.MustCompile(tc.expectedExpr)
		require.NotNil(t, expr)
		st, err := tc.stackTrace.MarshalText()
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(st), 0)
		require.True(t, expr.MatchString(string(st)))
	}
}

func TestStackTraceMarshalJSON(t *testing.T) {
	testcases := []struct {
		stackTrace
		expectedExpr string
	}{
		{
			[]frame{},
			"\\[\\]",
		},
		{
			nil,
			"\\[\\]",
		},
		{
			testStackTrace()[:2],
			"\\[({\"func\":\"github\\.com/benz9527/xboot/lib/infra\\..*\",\"fileAndLine\":\".*/lib/infra/.*\\.go:\\d+\"},?)+\\]",
		},
		{
			testStackTrace()[:2],
			"\\[[^]]+\\]",
		},
	}
	for _, tc := range testcases {
		expr := regexp.MustCompile(tc.expectedExpr)
		require.NotNil(t, expr)
		st, err := json.Marshal(tc.stackTrace)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(st), 0)
		require.True(t, expr.MatchString(string(st)))
	}
}

func TestStackFormat(t *testing.T) {
	c1 := getCallers(32)
	testcases := []struct {
		stack
		format       string
		expectedExpr string
	}{
		{
			[]uintptr{},
			"%+v",
			"stack:\nnil",
		},
		{
			nil,
			"%+v",
			"stack:\nnil",
		},
		{
			getCallers(32),
			"%+v",
			`stack:\n(.*\..*\n\t.*\.(go|s):\d+)+`,
		},
		{
			c1,
			"%+v",
			`stack:\n(.*\..*\n\t.*\.(go|s):\d+)+`,
		},
		{
			nil,
			"%#v",
			"infra\\.stack\\(infra\\.stackTrace\\(nil\\)\\)",
		},
		{
			getCallers(32),
			"%#v",
			`infra\.stack\(infra\.stackTrace\(\[(.*\.(go|s):\d+,?)+\]\)\)`,
		},
		{
			c1,
			"%#v",
			`infra\.stack\(infra\.stackTrace\(\[(.*\.(go|s):\d+,?)+\]\)\)`,
		},
	}
	for _, tc := range testcases {
		expr := regexp.MustCompile(tc.expectedExpr)
		require.NotNil(t, expr)
		s := fmt.Sprintf(tc.format, tc.stack)
		require.True(t, expr.MatchString(s))
	}
}

func TestErrorStackMarshalJSON(t *testing.T) {
	_errors := []error{
		fmt.Errorf("test"),
		fmt.Errorf("test2"),
	}
	es := AppendErrorStack(nil, _errors...)
	_bytes, err := json.Marshal(es)
	require.NoError(t, err)
	t.Log(string(_bytes))

	t.Logf("%v\n", es)
	t.Logf("%s\n", es)
	t.Logf("%+v\n", es)
	t.Logf("%#v\n", es)

	t.Log("=== 1 ===")
	es = AppendErrorStack(es, errors.New("test3"))
	_bytes, err = json.Marshal(es)
	require.NoError(t, err)
	t.Log(string(_bytes))

	t.Logf("%v\n", es)
	t.Logf("%s\n", es)
	t.Logf("%+v\n", es)
	t.Logf("%#v\n", es)

	t.Log("=== 2 ===")
	es = &errorStack{}
	_bytes, err = json.Marshal(es)
	require.NoError(t, err)
	t.Log(string(_bytes))

	t.Logf("%v\n", es)
	t.Logf("%s\n", es)
	t.Logf("%+v\n", es)
	t.Logf("%#v\n", es)

	t.Log("=== 3 ===")
	es = AppendErrorStack(es, errors.New("test4"))
	_bytes, err = json.Marshal(es)
	require.NoError(t, err)
	t.Log(string(_bytes))

	t.Logf("%v\n", es)
	t.Logf("%s\n", es)
	t.Logf("%+v\n", es)
	t.Logf("%#v\n", es)

	t.Log("=== 4 ===")
	es = AppendErrorStack(es, errors.New("test5"))
	_bytes, err = json.Marshal(es)
	require.NoError(t, err)
	t.Log(string(_bytes))

	t.Logf("%v\n", es)
	t.Logf("%s\n", es)
	t.Logf("%+v\n", es)
	t.Logf("%#v\n", es)

	t.Log("=== 5 ===")
	es = &errorStack{}
	es = WrapErrorStackWithMessage(es, "test6")
	_bytes, err = json.Marshal(es)
	require.NoError(t, err)
	t.Log(string(_bytes))

	t.Logf("%v\n", es)
	t.Logf("%s\n", es)
	t.Logf("%+v\n", es)
	t.Logf("%#v\n", es)

	t.Log("=== 6 ===")
	es = WrapErrorStackWithMessage(nil, "test7")
	_bytes, err = json.Marshal(es)
	require.NoError(t, err)
	t.Log(string(_bytes))

	t.Logf("%v\n", es)
	t.Logf("%s\n", es)
	t.Logf("%+v\n", es)
	t.Logf("%#v\n", es)

	t.Log("=== 7 ===")
	es = AppendErrorStack(nil, errors.New("test8"))
	_bytes, err = json.Marshal(es)
	require.NoError(t, err)
	t.Log(string(_bytes))

	t.Logf("%v\n", es)
	t.Logf("%s\n", es)
	t.Logf("%+v\n", es)
	t.Logf("%#v\n", es)

	es = WrapErrorStackWithMessage(nil, "")
	require.Nil(t, es)

	es = AppendErrorStack(nil)
	require.Nil(t, es)

	es = WrapErrorStack(nil)
	require.Nil(t, es)

	es = WrapErrorStack(errors.New("test9"))
	require.NotNil(t, es)

	es = AppendErrorStack(es, errors.New("test10"))
	require.NotNil(t, es)

	es = WrapErrorStackWithMessage(es, "test11")
	require.NotNil(t, es)

	es = AppendErrorStack(errors.New("test12"), errors.New("test13"))
	require.NotNil(t, es)

	es = WrapErrorStackWithMessage(errors.New("test14"), "test15")
	require.NotNil(t, es)

	es = WrapErrorStack(es)
	require.NotNil(t, es)
}
