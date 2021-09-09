package raftnode

import (
	"context"

	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
)

type httpRaft struct{ raft.Node }

func (h httpRaft) Process(ctx context.Context, m raftpb.Message) error {
	return h.Step(ctx, m)
}

func (h httpRaft) IsIDRemoved(_ uint64) bool {
	return false
}

var _ rafthttp.Raft = httpRaft{}
