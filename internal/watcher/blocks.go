package watcher

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/observer/common"
	"github.com/shutter-network/observer/common/utils"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/observer/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

type BlocksWatcher struct {
	config    *common.Config
	ethClient *ethclient.Client

	recentBlocksMux sync.Mutex
	recentBlocks    map[uint64]*types.Header
	mostRecentBlock uint64

	txMapper metrics.TxMapper
}

func NewBlocksWatcher(
	config *common.Config,
	ethClient *ethclient.Client,
	txMapper metrics.TxMapper,
) *BlocksWatcher {
	return &BlocksWatcher{
		config:          config,
		ethClient:       ethClient,
		recentBlocksMux: sync.Mutex{},
		recentBlocks:    make(map[uint64]*types.Header),
		mostRecentBlock: 0,
		txMapper:        txMapper,
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

	err := bw.txMapper.AddBlock(ctx, &data.Block{
		BlockHash:      header.Hash().Bytes(),
		BlockNumber:    header.Number.Int64(),
		BlockTimestamp: int64(header.Time),
		Slot:           int64(utils.GetSlotForBlock(header.Time, GenesisTimestamp, SlotDuration)),
	})
	if err != nil {
		log.Err(err).Msg("err adding block")
	}
	return err
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
