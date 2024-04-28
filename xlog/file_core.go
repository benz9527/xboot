package xlog

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ xLogCore = (*consoleCore)(nil)

type fileCore struct {
	core *commonCore
}

func (cc *fileCore) timeEncoder() zapcore.TimeEncoder   { return cc.core.tsEnc }
func (cc *fileCore) levelEncoder() zapcore.LevelEncoder { return cc.core.lvlEnc }
func (cc *fileCore) writeSyncer() zapcore.WriteSyncer   { return cc.core.ws }
func (cc *fileCore) outEncoder() func(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return cc.core.enc
}
func (cc *fileCore) context() context.Context             { return cc.core.ctx }
func (cc *fileCore) Enabled(lvl zapcore.Level) bool       { return cc.core.lvlEnabler.Enabled(lvl) }
func (cc *fileCore) With(fields []zap.Field) zapcore.Core { return cc.core.With(fields) }
func (cc *fileCore) Sync() error                          { return cc.core.Sync() }
func (cc *fileCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return cc.core.Check(ent, ce)
}

func (cc *fileCore) Write(ent zapcore.Entry, fields []zap.Field) error {
	return cc.core.Write(ent, fields)
}

type FileCoreConfig struct {
	FilePath                string `json:"filePath" yaml:"filePath"`
	Filename                string `json:"filename" yaml:"filename"`
	FileMaxSize             string `json:"fileMaxSize" yaml:"fileMaxSize"`
	FileMaxAge              string `json:"fileMaxAge" yaml:"fileMaxAge"`
	FileZipName             string `json:"fileZipName" yaml:"fileZipName"`
	FileBufferSize          string `json:"fileBufferSize" yaml:"fileBufferSize"`
	FileBufferFlushInterval int64  `json:"fileBufferFlushInterval" yaml:"fileBufferFlushInterval"` // Milliseconds
	FileMaxBackups          int    `json:"fileMaxBackups" yaml:"fileMaxBackups"`
	FileCompressBatch       int    `json:"fileCompressBatch" yaml:"fileCompressBatch"`
	FileCompressible        bool   `json:"fileCompressible" yaml:"fileCompressible"`
	FileRotateEnable        bool   `json:"fileRotateEnable" yaml:"fileRotateEnable"`
}

// TODO Runtime modification and applying.

func newFileCore(cfg *FileCoreConfig) XLogCoreConstructor {
	return func(
		ctx context.Context,
		lvlEnabler zapcore.LevelEnabler,
		encoder logEncoderType,
		lvlEnc zapcore.LevelEncoder,
		tsEnc zapcore.TimeEncoder,
	) xLogCore {
		// The root context is done and root writer will be close globally.
		if ctx == nil {
			return nil
		}

		if cfg == nil {
			cfg = &FileCoreConfig{
				Filename:         filepath.Base(os.Args[0]) + "_xlog.log",
				FilePath:         os.TempDir(),
				FileRotateEnable: false,
			}
		}

		var (
			err           error
			bufferEnabled = false
			bufSize       uint64
			bufInterval   int64
			fileWriter    io.WriteCloser
			ws            zapcore.WriteSyncer
		)
		if cfg.FileBufferSize != "" && cfg.FileBufferFlushInterval > 0 {
			bufSize, err = parseBufferSize(cfg.FileBufferSize)
			if err != nil {
				goto writerInit
			}
			if time.Duration(cfg.FileBufferFlushInterval).Milliseconds() < 200 {
				bufInterval = 200
			} else {
				if bufInterval = cfg.FileBufferFlushInterval; bufInterval > _maxBufferFlushMs {
					bufInterval = _maxBufferFlushMs
				}
			}
			bufferEnabled = true
		}
	writerInit:
		if cfg.FileRotateEnable {
			fileWriter = RotateLog(ctx, cfg)
		} else {
			fileWriter = SingleLog(ctx, cfg)
		}
		if bufferEnabled {
			ws = XLogBufferSyncer(ctx, fileWriter, bufSize, bufInterval)
		} else {
			ws = XLogLockSyncer(ctx, fileWriter)
		}

		cc := &fileCore{
			core: &commonCore{
				ctx:        ctx,
				lvlEnabler: lvlEnabler,
				lvlEnc:     lvlEnc,
				tsEnc:      tsEnc,
				ws:         ws,
				enc:        getEncoderByType(encoder),
			},
		}
		cfg := defaultCoreEncoderCfg()
		cfg.EncodeLevel = cc.core.lvlEnc
		cfg.EncodeTime = cc.core.tsEnc
		cc.core.core = zapcore.NewCore(cc.core.enc(cfg), cc.core.ws, cc.core.lvlEnabler)
		return cc
	}
}

const (
	_maxBufferSize    = 10 * MB
	_maxBufferFlushMs = 3000
)

func parseBufferSize(size string) (uint64, error) {
	_size, err := parseFileSize(size)
	if err != nil {
		return 0, err
	}
	if _size > uint64(_maxBufferSize) {
		return 0, errors.New("file buffer size too large")
	}
	return _size, nil
}
