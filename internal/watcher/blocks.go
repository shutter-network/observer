package watcher

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common"
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

func NewBlocksWatcher(config *common.Config, blocksChannel chan *BlockReceivedEvent) *BlocksWatcher {
	return &BlocksWatcher{
		config:        config,
		blocksChannel: blocksChannel,
	}
}

func (bw *BlocksWatcher) Start(ctx context.Context) error {
	errChan := make(chan error, 1)
	go func() {
		newHeads := make(chan *types.Header)
		sub, err := bw.ethClient.SubscribeNewHead(ctx, newHeads)
		if err != nil {
			errChan <- err
			return
		}
		defer sub.Unsubscribe()

		for {
			select {
			case <-ctx.Done():
				errChan <- err
				return
			case head := <-newHeads:
				bw.logNewHead(head)
				ev := &BlockReceivedEvent{
					Header: head,
					Time:   time.Now(),
				}
				bw.blocksChannel <- ev
			case err := <-sub.Err():
				errChan <- err
				return
			}
		}
	}()

	err := <-errChan
	return err
}

func (w *BlocksWatcher) logNewHead(head *types.Header) {
	log.Info().
		Int64("number", head.Number.Int64()).
		Hex("hash", head.Hash().Bytes()).
		Msg("new head")
}
