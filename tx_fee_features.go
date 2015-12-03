package main

import (
	"bytes"
	"container/list"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/boltdb/bolt"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const (
	featureBufferSize = 100
	// Generous estimate for number of txns in ablock.
	streamBufferSize   = 3000
	txRateSampleWindow = 10
	tableName          = "fee-features"
	columnFamily       = "tx-features"
)

// TxFeeFeatures....
type TxFeeFeature struct {
	Size                  int64
	Priority              float64
	TxFee                 btcutil.Amount
	TotalAncestralFees    btcutil.Amount
	FeePerKb              int64
	NumChildren           int
	NumParents            int
	MempoolSize           int
	MempoolSizeBytes      int
	BlockDiscovered       int32
	NumBlocksToConfirm    int32
	NumTxInLastBlock      int
	SecondsSinceLastBlock float64
	BlockDifficulty       float64
	IncomingTxRate        float64
	TxID                  *wire.ShaHash
	TotalInputValue       btcutil.Amount
	TotalOutputValue      btcutil.Amount
	LockTime              uint32
	TimeStamp             time.Time
	// TODO(all): add moving average of last N blocks worths of tx fees
}

// TODO(roasbeef): make sure all encoding is gucci
func (t *TxFeeFeature) GobEncode(w io.Writer) error {
	enc := gob.NewEncoder(w)

	if err := enc.Encode(t.Size); err != nil {
		return err
	}
	if err := enc.Encode(t.Priority); err != nil {
		return err
	}
	if err := enc.Encode(t.TxFee); err != nil {
		return err
	}
	if err := enc.Encode(t.TotalAncestralFees); err != nil {
		return err
	}
	if err := enc.Encode(t.FeePerKb); err != nil {
		return err
	}
	if err := enc.Encode(t.NumChildren); err != nil {
		return err
	}
	if err := enc.Encode(t.NumParents); err != nil {
		return err
	}
	if err := enc.Encode(t.MempoolSize); err != nil {
		return err
	}
	if err := enc.Encode(t.MempoolSizeBytes); err != nil {
		return err
	}
	if err := enc.Encode(t.BlockDiscovered); err != nil {
		return err
	}
	if err := enc.Encode(t.NumBlocksToConfirm); err != nil {
		return err
	}
	if err := enc.Encode(t.NumTxInLastBlock); err != nil {
		return err
	}
	if err := enc.Encode(t.SecondsSinceLastBlock); err != nil {
		return err
	}
	if err := enc.Encode(t.BlockDifficulty); err != nil {
		return err
	}
	if err := enc.Encode(t.IncomingTxRate); err != nil {
		return err
	}
	if err := enc.Encode(t.TxID); err != nil {
		return err
	}
	if err := enc.Encode(t.TotalInputValue); err != nil {
		return err
	}
	if err := enc.Encode(t.TotalOutputValue); err != nil {
		return err
	}
	if err := enc.Encode(t.LockTime); err != nil {
		return err
	}
	if err := enc.Encode(t.TimeStamp); err != nil {
		return err
	}

	return nil
}

func (t *TxFeeFeature) GobDecode(data io.Reader) error {
	dec := gob.NewDecoder(data)

	if err := dec.Decode(&t.Size); err != nil {
		return err
	}
	if err := dec.Decode(&t.Priority); err != nil {
		return err
	}
	if err := dec.Decode(&t.TxFee); err != nil {
		return err
	}
	if err := dec.Decode(&t.TotalAncestralFees); err != nil {
		return err
	}
	if err := dec.Decode(&t.FeePerKb); err != nil {
		return err
	}
	if err := dec.Decode(&t.NumChildren); err != nil {
		return err
	}
	if err := dec.Decode(&t.NumParents); err != nil {
		return err
	}
	if err := dec.Decode(&t.MempoolSize); err != nil {
		return err
	}
	if err := dec.Decode(&t.MempoolSizeBytes); err != nil {
		return err
	}
	if err := dec.Decode(&t.BlockDiscovered); err != nil {
		return err
	}
	if err := dec.Decode(&t.NumBlocksToConfirm); err != nil {
		return err
	}
	if err := dec.Decode(&t.NumTxInLastBlock); err != nil {
		return err
	}
	if err := dec.Decode(&t.SecondsSinceLastBlock); err != nil {
		return err
	}
	if err := dec.Decode(&t.BlockDifficulty); err != nil {
		return err
	}
	if err := dec.Decode(&t.IncomingTxRate); err != nil {
		return err
	}
	if err := dec.Decode(&t.TxID); err != nil {
		return err
	}
	if err := dec.Decode(&t.TotalInputValue); err != nil {
		return err
	}
	if err := dec.Decode(&t.TotalOutputValue); err != nil {
		return err
	}
	if err := dec.Decode(&t.LockTime); err != nil {
		return err
	}
	if err := dec.Decode(&t.TimeStamp); err != nil {
		return err
	}

	return nil
}

// featureAddMsg...
type featureInitMsg struct {
	feature *TxFeeFeature
}

// featureCompleteMsg...
type featureCompleteMsg struct {
	txIds       []wire.ShaHash
	difficulty  float64
	blockHeight int32
}

// txFeatureCollector...
type txFeatureCollector struct {
	sync.RWMutex

	txIDToFeature map[wire.ShaHash]*TxFeeFeature

	boundedTimeBuffer *list.List

	db *bolt.DB

	initFeatures     chan *featureInitMsg
	completeFeatures chan *featureCompleteMsg
	txAdd            chan time.Time

	sparkStreamChan   chan *TxFeeFeature
	streamingListener *net.TCPListener

	started  int32
	shutdown int32

	quit chan struct{}

	wg sync.WaitGroup
}

// newTxFeatureCollector...
// TODO(all): better name...
func newTxFeatureCollector(streamingPort int) (*txFeatureCollector, error) {
	db, err := bolt.Open("tx-features.db", 0600, nil)
	if err != nil {
		panic(err)
	}

	ip := net.ParseIP("0.0.0.0")
	if ip == nil {
		return nil, fmt.Errorf("Unable to parse ip!")
	}

	streamListener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: ip, Port: streamingPort})
	if err != nil {
		return nil, err

	}

	return &txFeatureCollector{
		db:                db,
		txIDToFeature:     make(map[wire.ShaHash]*TxFeeFeature),
		initFeatures:      make(chan *featureInitMsg, featureBufferSize),
		txAdd:             make(chan time.Time, featureBufferSize),
		completeFeatures:  make(chan *featureCompleteMsg, featureBufferSize),
		sparkStreamChan:   make(chan *TxFeeFeature, streamBufferSize),
		streamingListener: streamListener,
		quit:              make(chan struct{}),
		boundedTimeBuffer: list.New(),
	}, nil
}

func (n *txFeatureCollector) sparkConnectionHandler() {
	for {
		conn, err := n.streamingListener.AcceptTCP()
		if err != nil {
			peerLog.Errorf("FAILED TO GET SPARK CONN: %v", err)
			continue
		}

		go n.sparkStreamer(conn)
	}
	n.wg.Done()
}

func (n *txFeatureCollector) sparkStreamer(streamingConn *net.TCPConn) {
	peerLog.Infof("Got spark connection: %v", streamingConn)
out:
	for {
		select {
		case txFeature := <-n.sparkStreamChan:
			// Write the new collected feature out to our running
			// spark instance.
			// socketTextStream expects datum to be delimited by a new line.
			featureStr := txFeature.String() + "\n"
			streamingConn.Write([]byte(featureStr))
		case <-n.quit:
			break out
		}
	}
	n.wg.Done()
}

// collectionHandler...
func (n *txFeatureCollector) collectionHandler() {
	peerLog.Infof("handler started")
	now := time.Now()
	txsInLastBlock := 0
out:
	for {
		select {
		case t := <-n.txAdd:
			n.Lock()
			n.boundedTimeBuffer.PushBack(t)
			if n.boundedTimeBuffer.Len() > txRateSampleWindow {
				n.boundedTimeBuffer.Remove(n.boundedTimeBuffer.Front())
			}
			n.Unlock()
		case msg := <-n.initFeatures:
			peerLog.Infof("got feature init: %+v", msg.feature)
			n.txIDToFeature[*msg.feature.TxID] = msg.feature
		case msg := <-n.completeFeatures:
			prevNow := now
			timeDelta := time.Since(prevNow).Seconds()
			now = time.Now()
			// TODO(roasbeef): only if we're at the final main chain block
			peerLog.Infof("got feature complete, block %v", msg.blockHeight)
			if err := n.db.Update(func(tx *bolt.Tx) error {
				txBucket, err := tx.CreateBucketIfNotExists([]byte("txs"))
				if err != nil {
					peerLog.Errorf("unable to grab bucket : %v", err)
				}
				for _, txid := range msg.txIds {
					// The block might contain transactions that
					// have never entered out memepool.
					// TODO(roasbeef): Mutated transactions...
					if txFeature, ok := n.txIDToFeature[txid]; ok {
						delete(n.txIDToFeature, txid)

						txFeature.NumBlocksToConfirm = msg.blockHeight - txFeature.BlockDiscovered
						txFeature.NumTxInLastBlock = txsInLastBlock
						txFeature.SecondsSinceLastBlock = timeDelta
						txFeature.BlockDifficulty = msg.difficulty

						peerLog.Infof("writing feature %+v: ", txFeature)

						var b bytes.Buffer
						txFeature.GobEncode(&b)
						txBucket.Put(txFeature.TxID.Bytes(), b.Bytes())

						// Only send if the queue isn't currently
						// full.
						select {
						case n.sparkStreamChan <- txFeature:
							fmt.Println("spark chan full")
						default:
							fmt.Println("spark chan not full")
						}
					}
				}
				return nil
			}); err != nil {
				peerLog.Errorf("unable to write feature: %v", err)
			}
			txsInLastBlock = len(msg.txIds)
		case <-n.quit:
			break out
		}
	}

	n.wg.Done()
}

func (tx *TxFeeFeature) String() string {
	return fmt.Sprintf("%v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v",
		tx.Size, tx.Priority, int64(tx.TxFee), int64(tx.TotalAncestralFees),
		int64(tx.FeePerKb), tx.NumChildren, tx.NumParents, tx.MempoolSize, tx.MempoolSizeBytes,
		tx.BlockDiscovered, tx.NumBlocksToConfirm, tx.NumTxInLastBlock, tx.SecondsSinceLastBlock,
		tx.BlockDifficulty, tx.IncomingTxRate,
		int64(tx.TotalInputValue), int64(tx.TotalOutputValue), tx.LockTime,
		tx.TimeStamp.Unix())
}

// currentTxRate...
func (n *txFeatureCollector) currentTxRate() float64 {
	n.RLock()
	defer n.RUnlock()
	peerLog.Infof("len: %v", n.boundedTimeBuffer.Len())
	lastNode := n.boundedTimeBuffer.Back()
	firstNode := n.boundedTimeBuffer.Front()
	peerLog.Infof("first %v last %v", firstNode, lastNode)
	if lastNode != nil && firstNode != nil {
		rate := txRateSampleWindow / lastNode.Value.(time.Time).Sub(firstNode.Value.(time.Time)).Seconds()
		return rate
	} else {
		return 0
	}
}

// Start...
func (n *txFeatureCollector) Start() error {
	// Already started?
	if atomic.AddInt32(&n.started, 1) != 1 {
		return nil
	}

	n.wg.Add(2)
	go n.collectionHandler()
	go n.sparkConnectionHandler()

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
