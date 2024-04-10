package infra

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

var initPC = caller()

func caller() Frame {
	var PCs [3]uintptr
	n := runtime.Callers(2, PCs[:])
	frames := runtime.CallersFrames(PCs[:n])
	frame, _ := frames.Next()
	return Frame(frame.PC)
}

func TestFrameFormat(t *testing.T) {
	testcases := []struct {
		Frame
		format string
		want   string
	}{
		{
			initPC,
			"%s",
			"err_stack_test.go",
		},
		{
			initPC,
			"%+s",
			"github.com/benz9527/xboot/lib/infra.init\n\td:/Ben-Projs/Go/xboot/lib/infra/err_stack_test.go",
		},
		{
			initPC,
			"%n",
			"init",
		},
		{
			initPC,
			"%d",
			"13",
		},
		{
			initPC,
			"%v",
			"err_stack_test.go:13",
		},
		{
			initPC,
			"%+v",
			"github.com/benz9527/xboot/lib/infra.init\n\td:/Ben-Projs/Go/xboot/lib/infra/err_stack_test.go:13",
		},
		{
			Frame(0),
			"%s",
			"unknownFile",
		},
		{
			Frame(0),
			"%n",
			"unknownFunc",
		},
		{
			Frame(0),
			"%d",
			"0",
		},
	}

	for _, tc := range testcases {
		frameRes := fmt.Sprintf(tc.format, tc.Frame)
		require.Equal(t, tc.want, frameRes)
	}
}

func TestFrameMarshalText(t *testing.T) {
	testcases := []struct {
		Frame
		expected []byte
	}{
		{
			initPC,
			[]byte("github.com/benz9527/xboot/lib/infra.init d:/Ben-Projs/Go/xboot/lib/infra/err_stack_test.go:13"),
		},
		{
			Frame(0),
			[]byte("unknownFrame"),
		},
	}
	for _, tc := range testcases {
		_bytes, err := tc.Frame.MarshalText()
		require.NoError(t, err)
		require.Greater(t, len(_bytes), 0)
		require.True(t, bytes.Equal(_bytes, tc.expected))
	}
}

func TestFrameMarshalJSON(t *testing.T) {
	testcases := []struct {
		Frame
		expected []byte
	}{
		{
			initPC,
			[]byte("{\"func\":\"github.com/benz9527/xboot/lib/infra.init\",\"fileAndLine\":\"d:/Ben-Projs/Go/xboot/lib/infra/err_stack_test.go:13\"}"),
		},
		{
			Frame(0),
			[]byte("{\"frame\":\"unknownFrame\"}"),
		},
	}
	for _, tc := range testcases {
		_bytes, err := json.Marshal(tc.Frame)
		require.NoError(t, err)
		require.Greater(t, len(_bytes), 0)
		require.True(t, bytes.Equal(_bytes, tc.expected))
	}
}
