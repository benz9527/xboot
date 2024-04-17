package xlog

import (
	"context"
	"errors"
	randv2 "math/rand/v2"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	
	"github.com/benz9527/xboot/lib/infra"
)

func TestLogLevelString(t *testing.T) {
	require.Equal(t, "DEBUG", LogLevelDebug.String())
	require.Equal(t, "INFO", LogLevelInfo.String())
	require.Equal(t, "WARN", LogLevelWarn.String())
	require.Equal(t, "ERROR", LogLevelError.String())
	require.Equal(t, zapcore.DebugLevel, LogLevelDebug.zapLevel())
	require.Equal(t, zapcore.InfoLevel, LogLevelInfo.zapLevel())
	require.Equal(t, zapcore.WarnLevel, LogLevelWarn.zapLevel())
	require.Equal(t, zapcore.ErrorLevel, LogLevelError.zapLevel())
}

type testBanner struct{}

func (b testBanner) JSON() string {
	return "{\"app\":\"xboot\"}"
}

func (b *testBanner) PlainText() string {
	return `
██╗  ██╗██████╗  ██████╗  ██████╗ ████████╗
╚██╗██╔╝██╔══██╗██╔═══██╗██╔═══██╗╚══██╔══╝
 ╚███╔╝ ██████╔╝██║   ██║██║   ██║   ██║   
 ██╔██╗ ██╔══██╗██║   ██║██║   ██║   ██║   
██╔╝ ██╗██████╔╝╚██████╔╝╚██████╔╝   ██║   
╚═╝  ╚═╝╚═════╝  ╚═════╝  ╚═════╝    ╚═╝   
`
}

type testMemOutWriter struct {
	data []byte
}

func (w *testMemOutWriter) Write(p []byte) (n int, err error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *testMemOutWriter) Reset() {
	w.data = make([]byte, 0, 4096)
}

func TestLoggerPrintBanner(t *testing.T) {
	w := &testMemOutWriter{data: make([]byte, 0, 4096)}
	err := writerMap.Put(testMemAsOut, zapcore.AddSync(w))
	require.NoError(t, err)

	level := zapcore.DebugLevel
	cfg := zap.NewDevelopmentEncoderConfig()
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg),
		zapcore.Lock(os.Stdout),
		level,
	)
	l := zap.New(
		zapcore.NewTee(core),
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.AddCallerSkip(1), // Use caller filename as service
	)

	logger := &xLogger{
		writer: testMemAsOut,
	}
	logger.logger.Store(l)
	logger.Banner(&testBanner{})
	require.Equal(t, "{\"banner\":\"{\\\"app\\\":\\\"xboot\\\"}\"}\n", string(w.data))
	w.Reset()

	printBanner = sync.Once{}
	core = zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		zapcore.Lock(os.Stdout),
		level,
	)
	l = zap.New(
		zapcore.NewTee(core),
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.AddCallerSkip(1), // Use caller filename as service
	)
	logger = &xLogger{
		writer:  testMemAsOut,
		encoder: PlainText,
	}
	logger.logger.Store(l)
	logger.Banner(&testBanner{})
	require.Equal(t, `
██╗  ██╗██████╗  ██████╗  ██████╗ ████████╗
╚██╗██╔╝██╔══██╗██╔═══██╗██╔═══██╗╚══██╔══╝
 ╚███╔╝ ██████╔╝██║   ██║██║   ██║   ██║   
 ██╔██╗ ██╔══██╗██║   ██║██║   ██║   ██║   
██╔╝ ██╗██████╔╝╚██████╔╝╚██████╔╝   ██║   
╚═╝  ╚═╝╚═════╝  ╚═════╝  ╚═════╝    ╚═╝   

`, string(w.data))
	w.Reset()
}

type testObj1 struct {
	name string
	arr  []testObj2
	obj3 testObj3
}

type testObj2 struct {
	age int
}

type testObj3 struct {
	o float32
}

func TestXLogger_Zap_AllAPIs(t *testing.T) {
	testcases := []struct {
		name          string
		encoder       logEncoderType
		writer        logOutWriterType
		core          string
		defaultLogger bool
		ctxM          map[string]string
	}{
		{
			name:    "console json",
			encoder: JSON,
			writer:  StdOut,
			ctxM: map[string]string{
				"traceId": "TraceID",
				"service": "Svc",
			},
		},
		{
			name:    "console plaintext",
			encoder: PlainText,
			writer:  StdOut,
			core:    "console",
			ctxM: map[string]string{
				"traceId": "traceID",
				"service": "svc",
				"abc":     "",
			},
		},
		{
			name:          "console default json",
			defaultLogger: true,
		},
		{
			name:          "console default json2",
			defaultLogger: true,
			ctxM: map[string]string{
				"traceId": "",
				"service": "",
				"":        "",
				"abc":     "_",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var opts []XLoggerOption
			if tc.defaultLogger {
				opts = []XLoggerOption{}
			} else {
				opts = append(opts,
					WithXLoggerLevel(LogLevelDebug),
					WithXLoggerEncoder(tc.encoder),
					WithXLoggerWriter(tc.writer),
				)
			}
			if tc.ctxM != nil {
				for k, v := range tc.ctxM {
					opts = append(opts, WithXLoggerContextFieldExtract(k, v))
				}
			}
			if tc.core != "" {
				if tc.core == "console" {
					opts = append(opts, WithXLoggerConsoleCore())
				}
			}
			logger := NewXLogger(opts...)

			ctx := context.TODO()
			ctx = context.WithValue(ctx, "traceId", "1234567890")
			ctx = context.WithValue(ctx, "service", "xboot")

			logger.Debug("debug message 1")
			logger.DebugContext(ctx, "debug message 2")
			logger.Info("info message 1")
			logger.InfoContext(ctx, "info message 2")
			logger.Warn("warn message 1")
			logger.WarnContext(ctx, "warn message 2")
			err1 := infra.WrapErrorStack(errors.New("error 1"))
			logger.Error(err1, "error message 1")
			logger.ErrorContext(ctx, err1, "error message 2")
			logger.ErrorStack(err1, "error message 1")
			logger.ErrorStackContext(ctx, err1, "error message 2")

			obj1 := testObj1{
				name: "testObj1",
				arr: []testObj2{
					{age: 1},
					{age: 2},
				},
				obj3: testObj3{o: 3.14},
			}
			field := zap.Object("testObj1", zapcore.ObjectMarshalerFunc(
				func(oe zapcore.ObjectEncoder) error {
					oe.AddString("name", obj1.name)
					if err := oe.AddArray("arr", zapcore.ArrayMarshalerFunc(
						func(ae zapcore.ArrayEncoder) error {
							for _, v := range obj1.arr {
								if err := ae.AppendObject(zapcore.ObjectMarshalerFunc(func(enc zapcore.ObjectEncoder) error {
									enc.AddInt("age", v.age)
									return nil
								})); err != nil {
									return err
								}
							}
							return nil
						})); err != nil {
						return err
					}
					if err := oe.AddObject("obj3", zapcore.ObjectMarshalerFunc(
						func(oe zapcore.ObjectEncoder) error {
							oe.AddFloat32("o", obj1.obj3.o)
							return nil
						})); err != nil {
						return err
					}
					return nil
				}))
			logger.Info("info message 3", field)
			logger.InfoContext(ctx, "info message 4", field)

			logger.IncreaseLogLevel(zapcore.WarnLevel)
			require.Equal(t, zapcore.WarnLevel.String(), logger.Level())
			logger.Logf(getLogLevelOrDefault(""), "unprintable debug message 3")
			logger.Logf(getLogLevelOrDefault(LogLevelDebug.String()), "unprintable debug message 4")
			logger.Logf(getLogLevelOrDefault(LogLevelInfo.String()), "unprintable info message 5")
			logger.Logf(getLogLevelOrDefault(LogLevelWarn.String()), "printable warn message 3")
			logger.Logf(getLogLevelOrDefault(LogLevelError.String()), "printable error message 3")
			logger.ErrorStackf(err1, "error message 4")

			logger.IncreaseLogLevel(zapcore.DebugLevel)
			require.Equal(t, zapcore.DebugLevel.String(), logger.Level())
			logger.Logf(getLogLevelOrDefault(""), "dynamic printable debug message 4")
			logger.Logf(getLogLevelOrDefault(LogLevelDebug.String()), "dynamic printable debug message 5")
			logger.Logf(getLogLevelOrDefault(LogLevelInfo.String()), "dynamic printable info message 6")
			logger.Logf(getLogLevelOrDefault(LogLevelWarn.String()), "dynamic printable warn message 4")
			logger.Logf(getLogLevelOrDefault(LogLevelError.String()), "dynamic printable error message 4")
			logger.ErrorStackf(err1, "error message 5")

			logger.IncreaseLogLevel(zapcore.WarnLevel)
			require.Equal(t, zapcore.WarnLevel.String(), logger.Level())
			logger.Logf(getLogLevelOrDefault(""), "unprintable debug message 5")
			logger.Logf(getLogLevelOrDefault(LogLevelDebug.String()), "unprintable debug message 6")
			logger.Logf(getLogLevelOrDefault(LogLevelInfo.String()), "unprintable info message 7")
			logger.Logf(getLogLevelOrDefault(LogLevelWarn.String()), "printable warn message 5")
			logger.Logf(getLogLevelOrDefault(LogLevelError.String()), "printable error message 5")
			logger.ErrorStackf(err1, "error message 6")

			err := logger.Sync()
			if err != nil {
				t.Log(err)
			}
		})
	}
}

func TestXLogger_Zap_DataRace(t *testing.T) {
	logger := NewXLogger()
	lvls := []zapcore.Level{
		zapcore.DebugLevel,
		zapcore.InfoLevel,
		zapcore.WarnLevel,
		zapcore.ErrorLevel,
	}
	n := int32(len(lvls))
	var wg sync.WaitGroup
	total := 10
	wg.Add(total)
	for i := 0; i < total; i++ {
		go func(i int) {
			for j := 0; j < 100; j++ {
				rng := randv2.Int32N(n)
				if i*total+j == 666 {
					logger.IncreaseLogLevel(lvls[rng])
				}
				logger.Logf(lvls[rng], "message i: %d; j: %d", i, j)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	_ = logger.Sync()
}

func BenchmarkXLogger_Zap(b *testing.B) {
	logger := NewXLogger()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message")
	}
	b.ReportAllocs()
}
