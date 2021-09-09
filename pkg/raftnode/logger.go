package raftnode

import (
	"go.etcd.io/etcd/raft/v3"
	"go.uber.org/zap"
)

func newRaftLogger(lg *zap.Logger) raft.Logger {
	logger := lg.WithOptions(zap.AddCallerSkip(1))
	return &raftLogger{lg: logger, sugar: logger.Sugar()}
}

type raftLogger struct {
	lg    *zap.Logger
	sugar *zap.SugaredLogger
}

func (rl *raftLogger) Debug(args ...interface{}) {
	rl.sugar.Debug(args...)
}

func (rl *raftLogger) Debugf(format string, args ...interface{}) {
	rl.sugar.Debugf(format, args...)
}

func (rl *raftLogger) Error(args ...interface{}) {
	rl.sugar.Error(args...)
}

func (rl *raftLogger) Errorf(format string, args ...interface{}) {
	rl.sugar.Errorf(format, args...)
}

func (rl *raftLogger) Info(args ...interface{}) {
	rl.sugar.Info(args...)
}

func (rl *raftLogger) Infof(format string, args ...interface{}) {
	rl.sugar.Infof(format, args...)
}

func (rl *raftLogger) Warning(args ...interface{}) {
	rl.sugar.Warn(args...)
}

func (rl *raftLogger) Warningf(format string, args ...interface{}) {
	rl.sugar.Warnf(format, args...)
}

func (rl *raftLogger) Fatal(args ...interface{}) {
	rl.sugar.Fatal(args...)
}

func (rl *raftLogger) Fatalf(format string, args ...interface{}) {
	rl.sugar.Fatalf(format, args...)
}

func (rl *raftLogger) Panic(args ...interface{}) {
	rl.sugar.Panic(args...)
}

func (rl *raftLogger) Panicf(format string, args ...interface{}) {
	rl.sugar.Panicf(format, args...)
}
