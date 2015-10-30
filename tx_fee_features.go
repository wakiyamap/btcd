package main

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/cloud/bigtable"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const (
	featureBufferSize  = 100
	txRateSampleWindow = 10
	tableName          = "fee-features"
	columnFamily       = "tx-features"
)

// TxFeeFeatures....
type TxFeeFeature struct {
	Size               int64
	Priority           uint
	TxFee              btcutil.Amount
	NumChildren        int
	NumParents         int
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

func encodeToBytes(n uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, n)
	return buf
}

func Float64frombytes(bytes []byte) float64 {
	bits := binary.LittleEndian.Uint64(bytes)
	float := math.Float64frombits(bits)
	return float
}

func Float64bytes(float float64) []byte {
	bits := math.Float64bits(float)
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, bits)
	return bytes
}

func (txf *TxFeeFeature) WriteToBigTable(mut *bigtable.Mutation) {
	timeStamp := bigtable.Now()

	mut.Set(columnFamily, "size", timeStamp, encodeToBytes(uint64(txf.Size)))
	mut.Set(columnFamily, "priority", timeStamp, encodeToBytes(uint64(txf.Priority)))
	mut.Set(columnFamily, "txfee", timeStamp, encodeToBytes(uint64(txf.TxFee)))
	mut.Set(columnFamily, "num_children", timeStamp, encodeToBytes(uint64(txf.NumChildren)))
	mut.Set(columnFamily, "num_parents", timeStamp, encodeToBytes(uint64(txf.NumParents)))
	mut.Set(columnFamily, "mempool_size", timeStamp, encodeToBytes(uint64(txf.MempoolSize)))
	mut.Set(columnFamily, "mempool_size_bytes", timeStamp, encodeToBytes(uint64(txf.MempoolSizeBytes)))
	mut.Set(columnFamily, "time_since_last_block", timeStamp, encodeToBytes(uint64(txf.TimeSinceLastBlock)))
	mut.Set(columnFamily, "block_discovered", timeStamp, encodeToBytes(uint64(txf.BlockDiscovered)))
	mut.Set(columnFamily, "num_blocks_to_confirm", timeStamp, encodeToBytes(uint64(txf.NumBlocksToConfirm)))
	mut.Set(columnFamily, "block_difficulty", timeStamp, encodeToBytes(uint64(txf.BlockDifficulty)))
	mut.Set(columnFamily, "average_block_time", timeStamp, encodeToBytes(uint64(txf.AverageBlockTime)))
	mut.Set(columnFamily, "incoming_tx_rate", timeStamp, Float64bytes(txf.IncomingTxRate))
	mut.Set(columnFamily, "txid", timeStamp, txf.TxID.Bytes())
	var b bytes.Buffer
	txf.RawTx.Serialize(&b)
	mut.Set(columnFamily, "raw_tx", timeStamp, b.Bytes())
	mut.Set(columnFamily, "total_input_value", timeStamp, encodeToBytes(uint64(txf.TotalInputValue)))
	mut.Set(columnFamily, "total_output_value", timeStamp, encodeToBytes(uint64(txf.TotalOutputValue)))
	mut.Set(columnFamily, "lock_time", timeStamp, encodeToBytes(uint64(txf.LockTime)))
	mut.Set(columnFamily, "version", timeStamp, encodeToBytes(uint64(txf.Version)))
	mut.Set(columnFamily, "timestamp", timeStamp, encodeToBytes(uint64(timeStamp)))
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

	bigTable *bigtable.Client

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
func newTxFeatureCollector() (*txFeatureCollector, error) {
	bigTableAdmin, err := bigtable.NewAdminClient(context.Background(), "TransactionFeeFeatures",
		"us-central1-b", "transactionfeefeatures")
	if err != nil {
		return nil, err
	}

	// Create our table scheme if it doesn't exist already.
	tables, err := bigTableAdmin.Tables(context.Background())
	if err != nil {
		return nil, err
	}
	var tableExists bool
	for _, table := range tables {
		if table == tableName {
			tableExists = true
			break
		}
	}
	if !tableExists {
		if err := bigTableAdmin.CreateTable(context.Background(), tableName); err != nil {
			return nil, nil
		}
	}

	// Create our column family if it doesn't exist already.
	tblInfo, err := bigTableAdmin.TableInfo(context.Background(), tableName)
	if err != nil {
		return nil, err
	}
	var columnFamilyExists bool
	for _, colFam := range tblInfo.Families {
		if colFam == columnFamily {
			columnFamilyExists = true
			break
		}
	}
	if !columnFamilyExists {
		if err := bigTableAdmin.CreateColumnFamily(context.Background(), tableName, columnFamily); err != nil {
			return nil, err

		}
	}
	bigTableAdmin.Close()

	bigTable, err := bigtable.NewClient(context.Background(), "TransactionFeeFeatures",
		"us-central1-b", "transactionfeefeatures")
	if err != nil {
		return nil, err
	}

	return &txFeatureCollector{
		bigTable:         bigTable,
		txIDToFeature:    make(map[wire.ShaHash]*TxFeeFeature),
		initFeatures:     make(chan *featureInitMsg, featureBufferSize),
		txAdd:            make(chan time.Time),
		completeFeatures: make(chan *featureCompleteMsg, featureBufferSize), quit: make(chan struct{}),
		boundedTimeBuffer: list.New(),
	}, nil
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
			peerLog.Infof("got feature init")
			n.txIDToFeature[*msg.feature.TxID] = msg.feature
		case msg := <-n.completeFeatures:
			peerLog.Infof("got feature complete")
			for _, txid := range msg.txIds {
				// The block might contain transactions that
				// have never entered out memepool.
				// TODO(roasbeef): Mutated transactions...
				if txFeature, ok := n.txIDToFeature[txid]; ok {
					delete(n.txIDToFeature, txid)

					featureTable := n.bigTable.Open(tableName)
					mut := bigtable.NewMutation()

					txFeature.NumBlocksToConfirm = msg.blockHeight - txFeature.BlockDiscovered
					txFeature.WriteToBigTable(mut)
					if err := featureTable.Apply(context.Background(), txid.String(), mut); err != nil {
						peerLog.Warnf("unable to write to bigtable: %v", err)
					}
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

	rate := txRateSampleWindow / last.Sub(first).Seconds()
	peerLog.Infof("calculated rate as: %v", rate)
	return rate
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
