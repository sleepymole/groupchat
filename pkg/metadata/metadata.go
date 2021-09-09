package metadata

import (
	"encoding/json"

	"go.etcd.io/etcd/client/pkg/v3/types"
)

type Peer struct {
	ID  types.ID `json:"id"`
	URL string   `json:"url"`
}

type Metadata struct {
	ID    types.ID `json:"id"`
	Peers []Peer   `json:"peers"`
}

func (md *Metadata) MustMarshalJSON() []byte {
	data, err := json.Marshal(md)
	if err != nil {
		panic(err)
	}
	return data
}

func (md *Metadata) MustUnmarshalJSON(data []byte) {
	if err := json.Unmarshal(data, md); err != nil {
		panic(err)
	}
}
