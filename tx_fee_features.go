package main

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const (
	featureBufferSize  = 100
	txRateSampleWindow = 10
)

// TxFeeFeatures....
type TxFeeFeature struct {
	Size               int64
	Priority           uint
	TxFee              btcutil.Amount
	NumChildren        uint
	NumParents         uint
	MempoolSize        int
	MempoolSizeBytes   int
	TimeSinceLastBlock time.Duration
	BlockDiscovered    int32
	NumBlocksToConfirm int32
	BlockDifficulty    float64
	AverageBlockTime   time.Duration
	IncomingTxRate     float64
	TxID               *wire.ShaHash
	RawTx              *wire.MsgTx
	TotalInputValue    btcutil.Amount
	TotalOutputValue   btcutil.Amount
	LockTime           uint32
	Version            int32
	TimeStamp          time.Time
	// TODO(all): add moving average of last N blocks worths of tx fees
}

// featureAddMsg...
type featureInitMsg struct {
	feature *TxFeeFeature
}

// featureCompleteMsg...
type featureCompleteMsg struct {
	txIds       []wire.ShaHash
	blockHeight int32
}

// txFeatureCollector...
type txFeatureCollector struct {
	sync.RWMutex

	txIDToFeature map[wire.ShaHash]*TxFeeFeature

	boundedTimeBuffer *list.List

	initFeatures     chan *featureInitMsg
	completeFeatures chan *featureCompleteMsg
	txAdd            chan time.Time

	started  int32
	shutdown int32

	quit chan struct{}

	wg sync.WaitGroup
}

// newTxFeatureCollector...
// TODO(all): better name...
func newTxFeatureCollector() *txFeatureCollector {
	return &txFeatureCollector{
		txIDToFeature:     make(map[wire.ShaHash]*TxFeeFeature),
		initFeatures:      make(chan *featureInitMsg, featureBufferSize),
		txAdd:             make(chan time.Time),
		completeFeatures:  make(chan *featureCompleteMsg, featureBufferSize),
		quit:              make(chan struct{}),
		boundedTimeBuffer: list.New(),
	}
}

// collectionHandler...
func (n *txFeatureCollector) collectionHandler() {
out:
	for {
		select {
		case t := <-n.txAdd:
			n.Lock()
			n.boundedTimeBuffer.PushBack(t)
			if n.boundedTimeBuffer.Len() >= txRateSampleWindow {
				n.boundedTimeBuffer.Remove(n.boundedTimeBuffer.Front())
			}
			n.Unlock()
		case msg := <-n.initFeatures:
			n.txIDToFeature[*msg.feature.TxID] = msg.feature
		case msg := <-n.completeFeatures:
			for _, txid := range msg.txIds {
				// The block might contain transactions that
				// have never entered out memepool.
				if txFeature, ok := n.txIDToFeature[txid]; ok {
					txFeature.NumBlocksToConfirm = msg.blockHeight - txFeature.BlockDiscovered
					// TODO(roasbeef): write to big table...
					delete(n.txIDToFeature, txid)
				}
			}
		case <-n.quit:
			break out
		}
	}

	n.wg.Done()
}

// currentTxRate...
func (n *txFeatureCollector) currentTxRate() float64 {
	n.RLock()
	defer n.RUnlock()
	last := n.boundedTimeBuffer.Back().Value.(time.Time)
	first := n.boundedTimeBuffer.Front().Value.(time.Time)

	return txRateSampleWindow / last.Sub(first).Seconds()
}

// Start...
func (n *txFeatureCollector) Start() error {
	// Already started?
	if atomic.AddInt32(&n.started, 1) != 1 {
		return nil
	}

	n.wg.Add(1)
	go n.collectionHandler()

	return nil
}

// Stop....
func (n *txFeatureCollector) Stop() error {
	if atomic.AddInt32(&n.shutdown, 1) != 1 {
		return nil
	}

	close(n.quit)
	n.wg.Wait()
	return nil
}
