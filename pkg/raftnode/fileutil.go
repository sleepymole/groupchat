package raftnode

import (
	"os"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.uber.org/zap"
)

func ensureDir(lg *zap.Logger, dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		lg.Fatal("failed to create dir", zap.String("dir", dir), zap.Error(err))
	}
	if err := fileutil.IsDirWriteable(dir); err != nil {
		lg.Fatal("cannot write to dir", zap.String("dir", dir), zap.Error(err))
	}
}

func ensureEmptyDir(lg *zap.Logger, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		lg.Fatal("failed to remove all from dir", zap.String("dir", dir), zap.Error(err))
	}
	ensureDir(lg, dir)
}
