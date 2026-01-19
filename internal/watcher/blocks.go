package watcher

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/observer/common"
	"github.com/shutter-network/observer/common/utils"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/observer/internal/metrics"
	"github.com/shutter-network/observer/internal/syncer"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

type BlocksWatcher struct {
	config                     *common.Config
	ethClient                  *ethclient.Client
	txMapper                   metrics.TxMapper
	transactionSubmittedSyncer *syncer.TransactionSubmittedSyncer
	validatorRegistrySyncer    *syncer.ValidatorRegistrySyncer
	beaconClient               *http.Client

	recentBlocksMux sync.Mutex
	recentBlocks    map[uint64]*types.Header
	mostRecentBlock uint64
}

type BeaconBlockResponse struct {
	Data struct {
		Message struct {
			ProposerIndex string `json:"proposer_index"`
			Body          struct {
				Graffiti         string `json:"graffiti"`
				ExecutionPayload struct {
					BlockNumber string `json:"block_number"`
				} `json:"execution_payload"`
			} `json:"body"`
		} `json:"message"`
	} `json:"data"`
}

func NewBlocksWatcher(
	config *common.Config,
	ethClient *ethclient.Client,
	txMapper metrics.TxMapper,
	transactionSubmittedSyncer *syncer.TransactionSubmittedSyncer,
	validatorRegistrySyncer *syncer.ValidatorRegistrySyncer,
) *BlocksWatcher {
	return &BlocksWatcher{
		config:                     config,
		ethClient:                  ethClient,
		txMapper:                   txMapper,
		transactionSubmittedSyncer: transactionSubmittedSyncer,
		validatorRegistrySyncer:    validatorRegistrySyncer,
		beaconClient:               &http.Client{Timeout: 10 * time.Second},
		recentBlocksMux:            sync.Mutex{},
		recentBlocks:               make(map[uint64]*types.Header),
		mostRecentBlock:            0,
	}
}

func (bw *BlocksWatcher) Start(ctx context.Context, runner service.Runner) error {
	newHeads := make(chan *types.Header)
	sub, err := bw.ethClient.SubscribeNewHead(ctx, newHeads)
	if err != nil {
		return err
	}
	runner.Defer(sub.Unsubscribe)
	runner.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case head := <-newHeads:
				err := bw.processBlock(ctx, head)
				if err != nil {
					log.Err(err).Msg("err processing new block")
					return err
				}
				log.Info().
					Int64("number", head.Number.Int64()).
					Hex("hash", head.Hash().Bytes()).
					Msg("new head")
			case err := <-sub.Err():
				return err
			}
		}
	})

	return nil
}

func (bw *BlocksWatcher) processBlock(ctx context.Context, header *types.Header) error {
	err := bw.insertBlock(ctx, header)
	if err != nil {
		return err
	}
	bw.clearOldBlocks(header)

	if err := bw.transactionSubmittedSyncer.Sync(ctx, header); err != nil {
		return err
	}

	if err := bw.validatorRegistrySyncer.Sync(ctx, header); err != nil {
		return err
	}

	if err := bw.processGraffiti(ctx, header); err != nil {
		log.Error().
			Err(err).
			Uint64("execution client block_number", header.Number.Uint64()).
			Msg("failed to process graffiti; skipping")
	}

	epoch := utils.GetEpochForBlock(header.Time, GenesisTimestamp, SlotDuration, SlotsPerEpoch)
	if epoch > CurrentEpoch {
		CurrentEpoch = epoch
		nextEpoch := epoch + 1
		err := bw.txMapper.AddProposerDuties(ctx, nextEpoch)
		if err != nil {
			return err
		}
		log.Info().
			Uint64("current epoch", epoch).
			Uint64("next epoch", nextEpoch).
			Msg("new proposer duties added")
	}
	return nil
}

func (bw *BlocksWatcher) insertBlock(ctx context.Context, header *types.Header) error {
	bw.recentBlocksMux.Lock()
	defer bw.recentBlocksMux.Unlock()
	bw.recentBlocks[header.Number.Uint64()] = header
	if header.Number.Uint64() > bw.mostRecentBlock {
		bw.mostRecentBlock = header.Number.Uint64()
	}

	slot := int64(utils.GetSlotForBlock(header.Time, GenesisTimestamp, SlotDuration))

	err := bw.txMapper.AddBlock(ctx, &data.Block{
		BlockHash:      header.Hash().Bytes(),
		BlockNumber:    header.Number.Int64(),
		BlockTimestamp: int64(header.Time),
		Slot:           slot,
	})
	if err != nil {
		log.Err(err).Msg("err adding block")
		return err
	}

	err = bw.txMapper.AddSlotStatus(ctx, slot, data.SlotStatusValProposed)
	if err != nil {
		log.Err(err).Msg("err adding slot status")
		return err
	}
	return nil
}

func (bw *BlocksWatcher) clearOldBlocks(latestHeader *types.Header) {
	bw.recentBlocksMux.Lock()
	defer bw.recentBlocksMux.Unlock()

	tooOld := []uint64{}
	for block := range bw.recentBlocks {
		if block < latestHeader.Number.Uint64()-100 {
			tooOld = append(tooOld, block)
		}
	}
	for _, block := range tooOld {
		delete(bw.recentBlocks, block)
	}
}

func (bw *BlocksWatcher) getBlockHeaderFromSlot(slot uint64) (*types.Header, bool) {
	bw.recentBlocksMux.Lock()
	defer bw.recentBlocksMux.Unlock()

	slotTimestamp := utils.GetSlotTimestamp(slot, GenesisTimestamp, SlotDuration)
	if header, ok := bw.recentBlocks[bw.mostRecentBlock]; ok {
		if header.Time == slotTimestamp {
			return header, ok
		} else if header.Time < slotTimestamp {
			return nil, false
		}
	}

	for blockNumber := range bw.recentBlocks {
		if header, ok := bw.recentBlocks[blockNumber]; ok {
			if header.Time == slotTimestamp {
				return header, ok
			}
		}
	}

	return nil, false
}

func (bw *BlocksWatcher) processGraffiti(ctx context.Context, header *types.Header) error {
	if header.ParentBeaconRoot == nil {
		return fmt.Errorf("parent beacon root not available")
	}
	beaconRoot := header.ParentBeaconRoot.Hex()
	//when processing execution block N, the ParentBeaconBlockRoot points to the parent beacon block, so we are actually storing graffiti and proposer info for block N-1
	url := fmt.Sprintf("%s/eth/v1/beacon/blocks/%s", bw.config.BeaconAPIURL, beaconRoot)
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return err
	}

	res, err := bw.beaconClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get beacon block info: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("beacon block request failed: %s", res.Status)
	}

	var beaconResp BeaconBlockResponse

	if err := json.NewDecoder(res.Body).Decode(&beaconResp); err != nil {
		return fmt.Errorf("failed to decode beacon block JSON: %w", err)
	}

	validatorIndex, err := strconv.ParseInt(beaconResp.Data.Message.ProposerIndex, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid proposer index: %w", err)
	}

	graffitiBytes, err := hex.DecodeString(strings.TrimPrefix(beaconResp.Data.Message.Body.Graffiti, "0x"))
	if err != nil {
		return fmt.Errorf("failed to decode graffiti hex: %w", err)
	}

	graffiti := string(bytes.TrimRight(graffitiBytes, "\x00"))

	blockNumber, err := strconv.ParseInt(beaconResp.Data.Message.Body.ExecutionPayload.BlockNumber, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid block number in execution payload: %w", err)
	}

	upserted, err := bw.txMapper.UpsertGraffitiIfShutterized(ctx, validatorIndex, graffiti, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to upsert graffiti: %w", err)
	}

	if upserted {
		log.Info().
			Int64("validator_index", validatorIndex).
			Str("graffiti", graffiti).
			Int64("block_number", blockNumber).
			Msg("stored validator graffiti")
	} else {
		log.Debug().
			Int64("validator_index", validatorIndex).
			Msg("validator not shutterized; graffiti skipped")
	}

	return nil
}
