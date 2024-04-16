package xlog

import (
	"errors"
	"os"
	"testing"

	"go.uber.org/fx/fxevent"
	"go.uber.org/zap/zapcore"
)

func TestFxXLoggerAllCases(t *testing.T) {
	testcases := []struct {
		name  string
		event fxevent.Event
	}{
		{
			"onStartExecuting",
			&fxevent.OnStartExecuting{
				FunctionName: "testFunc1",
				CallerName:   "testCaller1",
			},
		},
		{
			"onStopExecuting",
			&fxevent.OnStartExecuting{
				FunctionName: "testFunc4",
				CallerName:   "testCaller4",
			},
		},
		{
			"onStopExecuted_err",
			&fxevent.OnStartExecuted{
				FunctionName: "testFunc5",
				CallerName:   "testCaller5",
				Runtime:      11,
				Err:          errors.New("fx error 2"),
			},
		},
		{
			"onStopExecuted_succ",
			&fxevent.OnStartExecuted{
				FunctionName: "testFunc6",
				CallerName:   "testCaller6",
				Runtime:      12,
			},
		},
		{
			"supplied_err",
			&fxevent.Supplied{
				TypeName:   "testType1",
				Err:        errors.New("fx error 3"),
				StackTrace: []string{"testStack1"},
			},
		},
		{
			"supplied_succ1",
			&fxevent.Supplied{
				TypeName:   "testType2",
				ModuleName: "testModule1",
			},
		},
		{
			"supplied_succ2",
			&fxevent.Supplied{
				TypeName: "testType3",
			},
		},
		{
			"provided_err",
			&fxevent.Provided{
				ConstructorName: "testConstructor1",
				Err:             errors.New("fx error 4"),
				StackTrace:      []string{"testStack2"},
			},
		},
		{
			"provided_succ1",
			&fxevent.Provided{
				OutputTypeNames: []string{"testType4"},
				ConstructorName: "testConstructor2",
				ModuleName:      "testModule2",
			},
		},
		{
			"provided_succ2",
			&fxevent.Provided{
				OutputTypeNames: []string{"testType5"},
				ConstructorName: "testConstructor3",
				Private:         true,
			},
		},
		{
			"replaced_err",
			&fxevent.Replaced{
				OutputTypeNames: []string{"testType4"},
				Err:             errors.New("fx error 5"),
				StackTrace:      []string{"testStack3"},
			},
		},
		{
			"replaced_succ1",
			&fxevent.Replaced{
				OutputTypeNames: []string{"testType5"},
				ModuleName:      "testModule3",
			},
		},
		{
			"replaced_succ2",
			&fxevent.Replaced{
				OutputTypeNames: []string{"testType6"},
			},
		},
		{
			"decorated_err",
			&fxevent.Decorated{
				DecoratorName: "testDecorator1",
				Err:           errors.New("fx error 6"),
				StackTrace:    []string{"testStack4"},
			},
		},
		{
			"decorated_succ1",
			&fxevent.Decorated{
				OutputTypeNames: []string{"testType7"},
				DecoratorName:   "testDecorator2",
				ModuleName:      "testModule4",
			},
		},
		{
			"decorated_succ2",
			&fxevent.Decorated{
				OutputTypeNames: []string{"testType8"},
				DecoratorName:   "testDecorator3",
			},
		},
		{
			"invoked_err",
			&fxevent.Invoked{
				FunctionName: "testFunc7",
				Err:          errors.New("fx error 7"),
				Trace:        "invoke trace 1",
			},
		},
		{
			"invoking_succ1",
			&fxevent.Invoking{
				FunctionName: "testFunc8",
				ModuleName:   "testModule5",
			},
		},
		{
			"invoking_succ2",
			&fxevent.Invoking{
				FunctionName: "testFunc9",
			},
		},
		{
			"stopping",
			&fxevent.Stopping{
				Signal: os.Kill,
			},
		},
		{
			"stopped",
			&fxevent.Stopped{
				Err: errors.New("fx error 8"),
			},
		},
		{
			"rollingBack",
			&fxevent.RollingBack{
				StartErr: errors.New("fx error 9"),
			},
		},
		{
			"rolledBack",
			&fxevent.RolledBack{
				Err: errors.New("fx error 10"),
			},
		},
		{
			"started_err",
			&fxevent.Started{
				Err: errors.New("fx error 11"),
			},
		},
		{
			"started_succ",
			&fxevent.Started{},
		},
		{
			"loggerInitialized_err",
			&fxevent.LoggerInitialized{
				Err: errors.New("fx error 12"),
			},
		},
		{
			"loggerInitialized_succ",
			&fxevent.LoggerInitialized{
				ConstructorName: "testConstructor4",
			},
		},
		{
			"onStopExecuted_err",
			&fxevent.OnStopExecuted{
				FunctionName: "testFunc10",
				CallerName:   "testCaller7",
				Runtime:      13,
				Err:          errors.New("fx error 13"),
			},
		},
		{
			"onStopExecuted_succ",
			&fxevent.OnStopExecuted{
				FunctionName: "testFunc11",
				CallerName:   "testCaller8",
				Runtime:      14,
			},
		},
		{
			"onStopExecuting",
			&fxevent.OnStopExecuting{
				FunctionName: "testFunc12",
				CallerName:   "testCaller9",
			},
		},
	}
	opts := []XLoggerOption{
		WithXLoggerLevel(LogLevelDebug),
		WithXLoggerEncoder(JSON),
		WithXLoggerWriter(StdOut),
		WithXLoggerConsoleCore(),
		WithXLoggerTimeEncoder(zapcore.ISO8601TimeEncoder),
		WithXLoggerLevelEncoder(zapcore.CapitalLevelEncoder),
	}
	logger := NewFxXLogger(NewXLogger(opts...))
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			logger.LogEvent(tc.event)
			_ = logger.logger.Sync()
		})
	}
}

func TestFxXLogger_ParentLogLevelChanged(t *testing.T) {
	var (
		parentLogger XLogger    = nil
		logger       *FxXLogger = nil
	)
	logger.LogEvent(&fxevent.LoggerInitialized{
		ConstructorName: "testConstructor4",
	})

	opts := []XLoggerOption{
		WithXLoggerLevel(LogLevelDebug),
		WithXLoggerEncoder(JSON),
		WithXLoggerWriter(StdOut),
		WithXLoggerConsoleCore(),
		WithXLoggerTimeEncoder(zapcore.ISO8601TimeEncoder),
		WithXLoggerLevelEncoder(zapcore.CapitalLevelEncoder),
	}
	parentLogger = NewXLogger(opts...)
	logger = NewFxXLogger(parentLogger)
	parentLogger.IncreaseLogLevel(zapcore.InfoLevel)
	parentLogger.Debug("abc")
	logger.LogEvent(&fxevent.LoggerInitialized{
		ConstructorName: "testConstructor4",
	})
	parentLogger.IncreaseLogLevel(zapcore.DebugLevel)
	parentLogger.Debug("abc")
	logger.LogEvent(&fxevent.LoggerInitialized{
		ConstructorName: "testConstructor4",
	})
	_ = parentLogger.Sync()
}
