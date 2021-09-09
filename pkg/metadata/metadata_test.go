package metadata

import (
	"encoding/hex"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/types"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randString() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

func TestEncoding(t *testing.T) {
	md := Metadata{
		ID: types.ID(rand.Uint64()),
	}
	for i := 0; i < 20; i++ {
		md.Peers = append(md.Peers, Peer{
			ID:  types.ID(rand.Uint64()),
			URL: randString(),
		})
	}
	data := md.MustMarshalJSON()
	var md2 Metadata
	md2.MustUnmarshalJSON(data)
	if !reflect.DeepEqual(md, md2) {
		t.Fatal("metadata has changed after marshaling then unmarshalling")
	}
}
