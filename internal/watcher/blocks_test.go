package watcher

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	"github.com/shutter-network/observer/common"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/observer/internal/metrics"
	"github.com/stretchr/testify/require"
)

func TestProcessGraffitiSuccess(t *testing.T) {
	parentRoot := ethcommon.HexToHash("0xabc123")
	expectedPath := "/eth/v1/beacon/blocks/" + parentRoot.Hex()
	beaconURL := "http://beacon.local"

	originalClient := http.DefaultClient
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})
	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, expectedPath, req.URL.Path)
			response := `{"data":{"message":{"proposer_index":"42","body":{"graffiti":"0x68656c6c6f0000","execution_payload":{"block_number":"789"}}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(response)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	mockMapper := &mockTxMapper{upsertResult: true}
	header := &types.Header{
		ParentBeaconRoot: &parentRoot,
	}

	bw := &BlocksWatcher{
		config:          &common.Config{BeaconAPIURL: beaconURL},
		txMapper:        mockMapper,
		beaconClient:    http.DefaultClient,
		recentBlocks:    make(map[uint64]*types.Header),
		recentBlocksMux: sync.Mutex{},
	}

	err := bw.processGraffiti(context.Background(), header)
	require.NoError(t, err)
	require.Equal(t, int64(42), mockMapper.validatorIndex)
	require.Equal(t, "hello", mockMapper.graffiti)
	require.Equal(t, int64(789), mockMapper.blockNumber)
	require.Equal(t, 1, mockMapper.callCount)
}

func TestProcessGraffitiBeaconError(t *testing.T) {
	parentRoot := ethcommon.HexToHash("0xdef456")
	beaconURL := "http://beacon.local"

	originalClient := http.DefaultClient
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})
	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/eth/v1/beacon/blocks/"+parentRoot.Hex(), req.URL.Path)
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	mockMapper := &mockTxMapper{}
	bw := &BlocksWatcher{
		config:          &common.Config{BeaconAPIURL: beaconURL},
		txMapper:        mockMapper,
		beaconClient:    http.DefaultClient,
		recentBlocks:    make(map[uint64]*types.Header),
		recentBlocksMux: sync.Mutex{},
	}

	err := bw.processGraffiti(context.Background(), &types.Header{ParentBeaconRoot: &parentRoot})
	require.Error(t, err)
	require.Zero(t, mockMapper.callCount)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (rt roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

type mockTxMapper struct {
	callCount      int
	validatorIndex int64
	graffiti       string
	blockNumber    int64
	upsertResult   bool
	err            error

	mux sync.Mutex
}

func (m *mockTxMapper) AddTransactionSubmittedEvent(ctx context.Context, tx pgx.Tx, st *sequencerBindings.SequencerTransactionSubmitted) error {
	panic("unexpected call to AddTransactionSubmittedEvent")
}

func (m *mockTxMapper) AddDecryptionKeysAndMessages(ctx context.Context, dkam *metrics.DecKeysAndMessages) error {
	panic("unexpected call to AddDecryptionKeysAndMessages")
}

func (m *mockTxMapper) AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error {
	panic("unexpected call to AddKeyShare")
}

func (m *mockTxMapper) AddBlock(ctx context.Context, b *data.Block) error {
	panic("unexpected call to AddBlock")
}

func (m *mockTxMapper) QueryBlockNumberFromValidatorRegistryEventsSyncedUntil(ctx context.Context) (int64, error) {
	panic("unexpected call to QueryBlockNumberFromValidatorRegistryEventsSyncedUntil")
}

func (m *mockTxMapper) AddValidatorRegistryEvent(ctx context.Context, tx pgx.Tx, vr *validatorRegistryBindings.ValidatorregistryUpdated) error {
	panic("unexpected call to AddValidatorRegistryEvent")
}

func (m *mockTxMapper) UpdateValidatorStatus(ctx context.Context) error {
	panic("unexpected call to UpdateValidatorStatus")
}

func (m *mockTxMapper) AddProposerDuties(ctx context.Context, epoch uint64) error {
	panic("unexpected call to AddProposerDuties")
}

func (m *mockTxMapper) UpsertGraffitiIfShutterized(ctx context.Context, validatorIndex int64, graffiti string, blockNumber int64) (bool, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	m.callCount++
	m.validatorIndex = validatorIndex
	m.graffiti = graffiti
	m.blockNumber = blockNumber
	return m.upsertResult, m.err
}
