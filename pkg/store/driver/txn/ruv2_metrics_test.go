// Copyright 2026 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package txn

import (
	"testing"

	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/tidb/pkg/util/execdetails"
	"github.com/stretchr/testify/require"
	"github.com/tikv/client-go/v2/tikvrpc"
)

func TestStorageProcessedKeysRUV2RPCInterceptor(t *testing.T) {
	ruv2Metrics := execdetails.NewRUV2Metrics()
	it := NewStorageProcessedKeysRUV2RPCInterceptor(ruv2Metrics)
	require.NotNil(t, it)

	readReq := &tikvrpc.Request{Type: tikvrpc.CmdBatchGet, StoreTp: tikvrpc.TiKV}
	writeReq := &tikvrpc.Request{Type: tikvrpc.CmdPrewrite, StoreTp: tikvrpc.TiKV}

	wrapFn := it.Wrap(func(_ string, req *tikvrpc.Request) (*tikvrpc.Response, error) {
		switch req.Type {
		case tikvrpc.CmdBatchGet:
			return &tikvrpc.Response{
				Resp: &kvrpcpb.BatchGetResponse{
					ExecDetailsV2: &kvrpcpb.ExecDetailsV2{
						RuV2: &kvrpcpb.RUV2{
							StorageProcessedKeysBatchGet: 9,
							StorageProcessedKeysGet:      1,
						},
					},
				},
			}, nil
		case tikvrpc.CmdPrewrite:
			return &tikvrpc.Response{Resp: &kvrpcpb.PrewriteResponse{}}, nil
		default:
			return &tikvrpc.Response{}, nil
		}
	})
	_, err := wrapFn("tikv-1", readReq)
	require.NoError(t, err)
	_, err = wrapFn("tikv-1", writeReq)
	require.NoError(t, err)

	snapshot := ruv2Metrics.Snapshot()
	require.Equal(t, int64(1), snapshot.ResourceManagerReadCnt)
	require.Equal(t, int64(1), snapshot.ResourceManagerWriteCnt)
	require.Equal(t, int64(9), snapshot.TiKVStorageProcessedKeysBatchGet)
	require.Equal(t, int64(1), snapshot.TiKVStorageProcessedKeysGet)
}

func TestStorageProcessedKeysRUV2RPCInterceptorNilMetrics(t *testing.T) {
	require.Nil(t, NewStorageProcessedKeysRUV2RPCInterceptor(nil))
}

func TestStorageProcessedKeysRUV2RPCInterceptorWithGetterFollowsCurrentStatement(t *testing.T) {
	metrics1 := execdetails.NewRUV2Metrics()
	metrics2 := execdetails.NewRUV2Metrics()
	current := metrics1

	it := NewStorageProcessedKeysRUV2RPCInterceptorWithGetter(func() *execdetails.RUV2Metrics {
		return current
	})
	require.NotNil(t, it)

	wrapFn := it.Wrap(func(_ string, req *tikvrpc.Request) (*tikvrpc.Response, error) {
		switch req.Type {
		case tikvrpc.CmdBatchGet:
			return &tikvrpc.Response{
				Resp: &kvrpcpb.BatchGetResponse{
					ExecDetailsV2: &kvrpcpb.ExecDetailsV2{
						RuV2: &kvrpcpb.RUV2{
							StorageProcessedKeysBatchGet: 2,
						},
					},
				},
			}, nil
		case tikvrpc.CmdPrewrite:
			return &tikvrpc.Response{Resp: &kvrpcpb.PrewriteResponse{}}, nil
		default:
			return &tikvrpc.Response{}, nil
		}
	})

	_, err := wrapFn("tikv-1", &tikvrpc.Request{Type: tikvrpc.CmdBatchGet, StoreTp: tikvrpc.TiKV})
	require.NoError(t, err)

	current = metrics2
	_, err = wrapFn("tikv-1", &tikvrpc.Request{Type: tikvrpc.CmdPrewrite, StoreTp: tikvrpc.TiKV})
	require.NoError(t, err)

	require.Equal(t, int64(1), metrics1.Snapshot().ResourceManagerReadCnt)
	require.Equal(t, int64(0), metrics1.Snapshot().ResourceManagerWriteCnt)
	require.Equal(t, int64(1), metrics2.Snapshot().ResourceManagerWriteCnt)
	require.Equal(t, int64(0), metrics2.Snapshot().ResourceManagerReadCnt)
}
