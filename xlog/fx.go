package xlog

import (
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type FxXLogger struct {
	logger XLogger
}

func (l *FxXLogger) LogEvent(event fxevent.Event) {
	if l == nil || l.logger == nil {
		return
	}

	switch e := event.(type) {
	case *fxevent.OnStartExecuting:
		l.logger.Debug("HOOK OnStart",
			zap.String("function", e.FunctionName),
			zap.String("caller", e.CallerName),
		)
	case *fxevent.OnStartExecuted:
		if e.Err != nil {
			l.logger.Error(e.Err, "HOOK OnStart failed",
				zap.String("function", e.FunctionName),
				zap.String("caller", e.CallerName),
				zap.Int64("in", int64(e.Runtime)),
			)
		} else {
			l.logger.Debug("HOOK OnStart successfully",
				zap.String("function", e.FunctionName),
				zap.String("caller", e.CallerName),
				zap.Int64("in", int64(e.Runtime)),
			)
		}
	case *fxevent.OnStopExecuting:
		l.logger.Info("Hook OnStop executing",
			zap.String("function", e.FunctionName),
			zap.String("caller", e.CallerName),
		)
	case *fxevent.OnStopExecuted:
		if e.Err != nil {
			l.logger.Error(e.Err, "HOOK OnStop executed failed",
				zap.String("function", e.FunctionName),
				zap.String("caller", e.CallerName),
				zap.Int64("in", int64(e.Runtime)),
			)
		} else {
			l.logger.Info("Hook OnStop executed ran successfully",
				zap.String("function", e.FunctionName),
				zap.String("caller", e.CallerName),
				zap.Int64("in", int64(e.Runtime)),
			)
		}
	case *fxevent.Supplied:
		if e.Err != nil {
			l.logger.Error(e.Err, "SUPPLY ERROR",
				zap.String("type", e.TypeName),
				zap.Strings("stacktrace", e.StackTrace),
			)
		} else if e.ModuleName != "" {
			l.logger.Debug("SUPPLY type from module",
				zap.String("type", e.TypeName),
				zap.String("module", e.ModuleName),
			)
		} else {
			l.logger.Debug("SUPPLY type only",
				zap.String("type", e.TypeName),
			)
		}
	case *fxevent.Provided:
		for _, rtype := range e.OutputTypeNames {
			if e.ModuleName != "" {
				l.logger.Debug("PROVIDE rtype from module",
					zap.Bool("PRIVATE", e.Private),
					zap.String("rtype", rtype),
					zap.String("constructor", e.ConstructorName),
					zap.String("module", e.ModuleName),
				)
			} else {
				l.logger.Debug("PROVIDE rtype from constructor",
					zap.Bool("PRIVATE", e.Private),
					zap.String("rtype", rtype),
					zap.String("constructor", e.ConstructorName),
				)
			}
		}
		if e.Err != nil {
			l.logger.Error(e.Err, "Error after options were applied",
				zap.Strings("stacktrace", e.StackTrace),
			)
		}
	case *fxevent.Replaced:
		for _, rtype := range e.OutputTypeNames {
			if e.ModuleName != "" {
				l.logger.Debug("REPLACE rtype from module",
					zap.String("rtype", rtype),
					zap.String("module", e.ModuleName),
				)
			} else {
				l.logger.Debug("REPLACE rtype",
					zap.String("rtype", rtype),
				)
			}
		}
		if e.Err != nil {
			l.logger.Error(e.Err, "ERROR Failed to replace",
				zap.Strings("stacktrace", e.StackTrace),
			)
		}
	case *fxevent.Decorated:
		for _, rtype := range e.OutputTypeNames {
			if e.ModuleName != "" {
				l.logger.Debug("DECORATE rtype from module",
					zap.String("rtype", rtype),
					zap.String("decorate", e.DecoratorName),
					zap.String("module", e.ModuleName),
				)
			} else {
				l.logger.Debug("DECORATE rtype",
					zap.String("rtype", rtype),
					zap.String("decorate", e.DecoratorName),
				)
			}
		}
		if e.Err != nil {
			l.logger.Error(e.Err, "Error after decorated options were applied",
				zap.Strings("stacktrace", e.StackTrace),
			)
		}
	case *fxevent.Invoking:
		if e.ModuleName != "" {
			l.logger.Debug("INVOKING function from module",
				zap.String("function", e.FunctionName),
				zap.String("module", e.ModuleName),
			)
		} else {
			l.logger.Debug("INVOKING", zap.String("function", e.FunctionName))
		}
	case *fxevent.Invoked:
		if e.Err != nil {
			l.logger.Error(e.Err, "Error fx.Invoke",
				zap.String("function", e.FunctionName),
				zap.String("trace", e.Trace),
			)
		}
	case *fxevent.Stopping:
		l.logger.Info("STOPPING", zap.String("signal", e.Signal.String()))
	case *fxevent.Stopped:
		if e.Err != nil {
			l.logger.Error(e.Err, "Failed to stop cleanly")
		}
	case *fxevent.RollingBack:
		l.logger.Warn("Start failed, rolling back",
			zap.Error(e.StartErr),
		)
	case *fxevent.RolledBack:
		if e.Err != nil {
			l.logger.Error(e.Err, "Couldn't roll back cleanly")
		}
	case *fxevent.Started:
		if e.Err != nil {
			l.logger.Error(e.Err, "Failed to start")
		} else {
			l.logger.Debug("RUNNING")
		}
	case *fxevent.LoggerInitialized:
		if e.Err != nil {
			l.logger.Error(e.Err, "Failed to initialize custom logger")
		} else {
			l.logger.Debug("LOGGER Initialized custom logger", zap.String("constructor", e.ConstructorName))
		}
	}
}

func NewFxXLogger(logger XLogger) *FxXLogger {
	l := &xLogger{}
	l.logger.Store(logger.
		zap().
		Named("Fx").
		WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			if core == nil {
				panic("[XLogger] core is nil")
			}
			cc, ok := core.(xLogCore)
			if !ok {
				panic("[XLogger] core is not XLogCore")
			}
			var err error
			if mc, ok := cc.(*xLogMultiCore); ok && mc != nil {
				if cc, err = WrapCores(mc.cores, componentCoreEncoderCfg); err != nil {
					panic(err)
				}
			} else {
				if cc, err = WrapCore(cc, componentCoreEncoderCfg); err != nil {
					panic(err)
				}
			}
			return cc
		})),
	)
	return &FxXLogger{logger: l}
}
