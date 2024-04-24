package xlog

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	mock "github.com/DATA-DOG/go-sqlmock"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

func genDBMock(logger glogger.Interface) (*gorm.DB, mock.Sqlmock, error) {
	db, mock, err := mock.New()
	if err != nil {
		return nil, nil, err
	}
	// Mock SQLite3 DB connection, the sqlite version query is essential for go-sqlite driver.
	mock.ExpectQuery(`select sqlite_version()`).
		WithArgs().
		WillReturnRows(mock.NewRows([]string{"sqlite_version()"}).
			AddRow("3.38.0"))
	gdb, err := gorm.Open(sqlite.Dialector{
		DriverName: sqlite.DriverName,
		Conn:       db, // DSN is free. IP, port, username and password is free too.
	}, &gorm.Config{
		Logger: logger,
	})
	if err != nil {
		return nil, nil, err
	}
	// gdb is gorm db connection to the sql mock.
	return gdb, mock, nil
}

func TestGormXLogger_Sqlite3(t *testing.T) {
	var (
		parentLogger XLogger      = nil
		logger       *GormXLogger = nil
	)
	opts := []XLoggerOption{
		WithXLoggerLevel(LogLevelDebug),
		WithXLoggerEncoder(JSON),
		WithXLoggerWriter(StdOut),
		WithXLoggerConsoleCore(),
		WithXLoggerTimeEncoder(zapcore.ISO8601TimeEncoder),
		WithXLoggerLevelEncoder(zapcore.CapitalLevelEncoder),
	}
	parentLogger = NewXLogger(opts...)
	logger = NewGormXLogger(parentLogger,
		WithGormXLoggerIgnoreRecord404Err(),
		WithGormXLoggerLogLevel(glogger.Info),
		WithGormXLoggerSlowThreshold(200*time.Millisecond),
	)

	db, mock, err := genDBMock(logger)
	require.NoError(t, err)

	type fields struct {
		client *gorm.DB
	}
	type args struct {
	}
	testcases := []struct {
		name   string
		fields fields
		args   args
		invoke func(args)
		exec   func(*testing.T, args, *gorm.DB)
	}{
		{
			name: "create tbl",
			fields: fields{
				client: db,
			},
			args: args{},
			invoke: func(args args) {
				// Mock the SQL by pattern string with dynamic value to match gorm SQL request to be executed really.
				// Mock the db transaction start.
				mock.ExpectBegin().WillReturnError(nil)
				mock.ExpectExec(`SAVEPOINT create-obj`).WithArgs().WillReturnResult(driver.ResultNoRows)
				// Mock the db transaction commit without error.
				mock.ExpectCommit().WillReturnError(nil)
			},
			exec: func(tt *testing.T, args args, client *gorm.DB) {
				sp := "create-obj"
				tx := client.Begin(&sql.TxOptions{
					Isolation: sql.LevelDefault,
					ReadOnly:  false,
				}).SavePoint(sp)
				err := tx.Commit().Error
				require.NoError(tt, err)
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			tc.invoke(tc.args)
			tc.exec(tt, tc.args, tc.fields.client)
		})
	}
	_ = parentLogger.Sync()
}

func TestGormXLogger_AllAPIs(t *testing.T) {
	var (
		parentLogger XLogger      = nil
		logger       *GormXLogger = nil
	)
	opts := []XLoggerOption{
		WithXLoggerLevel(LogLevelDebug),
		WithXLoggerEncoder(JSON),
		WithXLoggerWriter(StdOut),
		WithXLoggerConsoleCore(),
		WithXLoggerTimeEncoder(zapcore.ISO8601TimeEncoder),
		WithXLoggerLevelEncoder(zapcore.CapitalLevelEncoder),
	}
	parentLogger = NewXLogger(opts...)
	logger = NewGormXLogger(parentLogger,
		WithGormXLoggerIgnoreRecord404Err(),
		WithGormXLoggerLogLevel(glogger.Info),
		WithGormXLoggerParameterizedQueries(),
	)

	require.Equal(t, zap.ErrorLevel, getLogLevelOrDefaultForGorm(glogger.Error))
	require.Equal(t, zap.WarnLevel, getLogLevelOrDefaultForGorm(glogger.Warn))
	require.Equal(t, zap.InfoLevel, getLogLevelOrDefaultForGorm(glogger.Info))
	require.Equal(t, zap.DebugLevel, getLogLevelOrDefaultForGorm(glogger.Silent))

	logger.Info(context.TODO(), "sql %s", "insert into abc values(1,2,3)")
	logger.Warn(context.TODO(), "sql %s", "insert into abc values(1,2,3)")
	logger.Error(context.TODO(), "sql %s", "insert into abc values(1,2,3)")
	logger.Trace(context.TODO(), time.Now(), func() (string, int64) {
		return "insert into abc values(1,2,3)", -1
	}, nil)
	logger.Trace(context.TODO(), time.Now(), func() (string, int64) {
		return "insert into abc values(1,2,3)", 1
	}, nil)
	logger.Trace(context.TODO(), time.Now(), func() (string, int64) {
		return "insert into abc values(1,2,3)", -1
	}, errors.New("insert error"))
	logger.Trace(context.TODO(), time.Now(), func() (string, int64) {
		return "insert into abc values(1,2,3)", 1
	}, errors.New("insert error"))
	logger.Trace(context.TODO(), time.Now().Add(-600*time.Millisecond), func() (string, int64) {
		return "insert into abc values(1,2,3)", -1
	}, nil)
	logger.Trace(context.TODO(), time.Now().Add(-550*time.Millisecond), func() (string, int64) {
		return "insert into abc values(1,2,3)", 1
	}, nil)
	_ = parentLogger.Sync()
	logger.LogMode(glogger.Silent).Trace(context.TODO(), time.Now().Add(-500*time.Millisecond), func() (string, int64) {
		return "insert into abc values(1,2,3)", 1
	}, nil)
	_ = parentLogger.Sync()
}
