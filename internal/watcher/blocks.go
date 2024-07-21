package watcher

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

type BlocksWatcher struct {
	config        *common.Config
	blocksChannel chan *BlockReceivedEvent
	ethClient     *ethclient.Client
}

type BlockReceivedEvent struct {
	Header *types.Header
	Time   time.Time
}

func NewBlocksWatcher(config *common.Config, blocksChannel chan *BlockReceivedEvent, ethClient *ethclient.Client) *BlocksWatcher {
	return &BlocksWatcher{
		config:        config,
		blocksChannel: blocksChannel,
		ethClient:     ethClient,
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
				log.Info().
					Int64("number", head.Number.Int64()).
					Hex("hash", head.Hash().Bytes()).
					Msg("new head")
				ev := &BlockReceivedEvent{
					Header: head,
					Time:   time.Now(),
				}
				bw.blocksChannel <- ev
			case err := <-sub.Err():
				return err
			}
		}
	})

	return nil
}
