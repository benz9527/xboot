package xlog

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	glogger "gorm.io/gorm/logger"
	gutils "gorm.io/gorm/utils"
)

var _ glogger.Interface = (*GormXLogger)(nil)

type GormXLogger struct {
	logger              XLogger
	cfg                 *glogger.Config
	dynamicLevelEnabler zap.AtomicLevel
	gormLevel           int32
}

func (l *GormXLogger) LogMode(lvl glogger.LogLevel) glogger.Interface {
	atomic.StoreInt32(&l.gormLevel, int32(lvl))
	l.dynamicLevelEnabler.SetLevel(getLogLevelOrDefaultForGorm(lvl))
	return l
}

func (l *GormXLogger) Info(ctx context.Context, msg string, data ...any) {
	if glogger.LogLevel(atomic.LoadInt32(&l.gormLevel)) >= glogger.Info {
		l.logger.InfoContext(ctx, fmt.Sprintf(msg, data...), zap.String("fileAndLine", gutils.FileWithLineNum()))
	}
}

func (l *GormXLogger) Warn(ctx context.Context, msg string, data ...any) {
	if glogger.LogLevel(atomic.LoadInt32(&l.gormLevel)) >= glogger.Warn {
		l.logger.WarnContext(ctx, fmt.Sprintf(msg, data...), zap.String("fileAndLine", gutils.FileWithLineNum()))
	}
}

func (l *GormXLogger) Error(ctx context.Context, msg string, data ...any) {
	if glogger.LogLevel(atomic.LoadInt32(&l.gormLevel)) >= glogger.Error {
		l.logger.ErrorContext(ctx, nil, fmt.Sprintf(msg, data...), zap.String("fileAndLine", gutils.FileWithLineNum()))
	}
}

func (l *GormXLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if glogger.LogLevel(atomic.LoadInt32(&l.gormLevel)) <= glogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && glogger.LogLevel(atomic.LoadInt32(&l.gormLevel)) >= glogger.Error && (!errors.Is(err, glogger.ErrRecordNotFound) || l.cfg.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows <= -1 {
			l.logger.ErrorContext(ctx, err, "error trace",
				zap.String("fileAndLine", gutils.FileWithLineNum()),
				zap.String("rows", "-"),
				zap.Int64("elapsedMs", elapsed.Milliseconds()),
				zap.String("sql", sql),
			)
		} else {
			l.logger.ErrorContext(ctx, err, "error trace",
				zap.String("fileAndLine", gutils.FileWithLineNum()),
				zap.String("rows", strconv.FormatInt(rows, 10)),
				zap.Int64("elapsedMs", elapsed.Milliseconds()),
				zap.String("sql", sql),
			)
		}
	case elapsed > l.cfg.SlowThreshold && l.cfg.SlowThreshold != 0 && glogger.LogLevel(atomic.LoadInt32(&l.gormLevel)) >= glogger.Warn:
		sql, rows := fc()
		if rows <= -1 {
			l.logger.WarnContext(ctx, "slow sql",
				zap.Int64("thresholdMs", l.cfg.SlowThreshold.Milliseconds()),
				zap.String("fileAndLine", gutils.FileWithLineNum()),
				zap.String("rows", "-"),
				zap.Int64("elapsedMs", elapsed.Milliseconds()),
				zap.String("sql", sql),
			)
		} else {
			l.logger.WarnContext(ctx, "slow sql",
				zap.Int64("thresholdMs", l.cfg.SlowThreshold.Milliseconds()),
				zap.String("fileAndLine", gutils.FileWithLineNum()),
				zap.String("rows", strconv.FormatInt(rows, 10)),
				zap.Int64("elapsedMs", elapsed.Milliseconds()),
				zap.String("sql", sql),
			)
		}
	case glogger.LogLevel(atomic.LoadInt32(&l.gormLevel)) == glogger.Info:
		sql, rows := fc()
		if rows <= -1 {
			l.logger.InfoContext(ctx, "common sql info",
				zap.String("fileAndLine", gutils.FileWithLineNum()),
				zap.String("rows", "-"),
				zap.Int64("elapsedMs", elapsed.Milliseconds()),
				zap.String("sql", sql),
			)
		} else {
			l.logger.InfoContext(ctx, "common sql info",
				zap.String("fileAndLine", gutils.FileWithLineNum()),
				zap.String("rows", strconv.FormatInt(rows, 10)),
				zap.Int64("elapsedMs", elapsed.Milliseconds()),
				zap.String("sql", sql),
			)
		}
	}
}

func NewGormXLogger(logger XLogger, opts ...GormXLoggerOption) *GormXLogger {
	gl := &GormXLogger{
		cfg: &glogger.Config{},
	}
	for _, o := range opts {
		o(gl.cfg)
	}
	if gl.cfg.SlowThreshold <= 0 {
		gl.cfg.SlowThreshold = 500 * time.Millisecond
	}
	gl.gormLevel = int32(gl.cfg.LogLevel)
	gl.dynamicLevelEnabler = zap.NewAtomicLevelAt(getLogLevelOrDefaultForGorm(gl.cfg.LogLevel))

	l := &xLogger{}
	l.logger.Store(logger.
		zap().
		Named("Gorm").
		WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			if core == nil {
				panic("[XLogger] core is nil")
			}
			cc, ok := core.(XLogCore)
			if !ok {
				panic("[XLogger] core is not XLogCore")
			}
			var err error
			if cc, err = WrapCoreNewLevelEnabler(cc,
				gl.dynamicLevelEnabler,
				componentCoreEncoderCfg); err != nil {
				panic(err)
			}
			return cc
		})),
	)
	gl.logger = l
	return gl
}

func getLogLevelOrDefaultForGorm(lvl glogger.LogLevel) zapcore.Level {
	switch lvl {
	case glogger.Info:
		return zapcore.InfoLevel
	case glogger.Warn:
		return zapcore.WarnLevel
	case glogger.Error:
		return zapcore.ErrorLevel
	case glogger.Silent:
		fallthrough
	default:
		return zapcore.DebugLevel
	}
}

type GormXLoggerOption func(*glogger.Config)

func WithGormXLoggerSlowThreshold(threshold time.Duration) GormXLoggerOption {
	return func(cfg *glogger.Config) {
		cfg.SlowThreshold = threshold
	}
}

func WithGormXLoggerLogLevel(lvl glogger.LogLevel) GormXLoggerOption {
	return func(cfg *glogger.Config) {
		cfg.LogLevel = lvl
	}
}

func WithGormXLoggerIgnoreRecord404Err() GormXLoggerOption {
	return func(cfg *glogger.Config) {
		cfg.IgnoreRecordNotFoundError = true
	}
}

func WithGormXLoggerParameterizedQueries() GormXLoggerOption {
	return func(cfg *glogger.Config) {
		cfg.ParameterizedQueries = true
	}
}
