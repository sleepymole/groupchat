package main

import (
	_ "net/http/pprof"
	"os"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/gozssky/groupchat/pkg/app"
)

var (
	flagPort     = kingpin.Flag("port", "Port to listen for client requests").Default("8080").Int()
	flagRaftPort = kingpin.Flag("raft-port", "Port to listen for peer raft messages").Default("8081").Int()
	flagDataDir  = kingpin.Flag("data-dir", "Data directory to store snapshot and WAL logs.").Default("/tmp/groupchat").String()
	flagLogLevel = kingpin.Flag("log-level", "Log level.").Default("info").Enum("debug", "info", "warn", "error")
)

func main() {
	kingpin.Parse()

	logEncCfg := zap.NewProductionEncoderConfig()
	logEncCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	logEncCfg.EncodeDuration = zapcore.StringDurationEncoder
	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig = logEncCfg
	if err := logCfg.Level.UnmarshalText([]byte(*flagLogLevel)); err != nil {
		panic(err)
	}
	logger, err := logCfg.Build()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(logger)

	logger.Info("starting chat server", zap.Int("port", *flagPort), zap.String("data-dir", *flagDataDir))

	if err := os.MkdirAll(*flagDataDir, 0755); err != nil {
		logger.Fatal("failed to create data dir", zap.Error(err))
	}
	if err := fileutil.IsDirWriteable(*flagDataDir); err != nil {
		logger.Fatal("data dir is not writable", zap.Error(err))
	}
	if err := os.MkdirAll(*flagDataDir, 0755); err != nil {
		logger.Fatal("failed to create data dir", zap.Error(err))
	}
	srv := app.NewServer(logger, *flagPort, *flagRaftPort, *flagDataDir)
	if err := srv.Run(); err != nil {
		logger.Fatal("failed to run server", zap.Error(err))
	}
}
