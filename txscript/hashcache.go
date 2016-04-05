// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"sync"

	"github.com/btcsuite/btcd/wire"
)

// TxSigHashes..
// TODO(roasbeef):
//  * mru cache, or rolling bloom filter?
type TxSigHashes struct {
	HashPrevOuts wire.ShaHash
	HashSequence wire.ShaHash
	HashOutputs  wire.ShaHash
}

func NewTxSigHashes(tx *wire.MsgTx) *TxSigHashes {
	return &TxSigHashes{
		HashPrevOuts: calcHashPrevOuts(tx),
		HashSequence: calcHashSequence(tx),
		HashOutputs:  calcHashOutputs(tx),
	}
}

// HashCache...
// TODO(roasbeef): fin
type HashCache struct {
	sigHashes map[wire.ShaHash]*TxSigHashes

	sync.RWMutex
}

// NewHashCache...
func NewHashCache(maxSize uint) *HashCache {
	return &HashCache{
		sigHashes: make(map[wire.ShaHash]*TxSigHashes, maxSize),
	}
}

func (h *HashCache) AddSigHashes(tx *wire.MsgTx) {
	h.Lock()
	defer h.Unlock()

	// TODO(roasbeef): eviction scheme?

	sigHashes := NewTxSigHashes(tx)

	txid := tx.TxSha()
	h.sigHashes[txid] = sigHashes

	return
}

func (h *HashCache) ContainsHashes(tx *wire.MsgTx) bool {
	h.RLock()
	defer h.RUnlock()

	txid := tx.TxSha()
	_, found := h.sigHashes[txid]
	return found
}

func (h *HashCache) GetSigHashes(tx *wire.MsgTx) (*TxSigHashes, bool) {
	h.RLock()
	defer h.RUnlock()

	// TODO(roasbeef): just populate if not found?
	txid := tx.TxSha()
	item, found := h.sigHashes[txid]

	return item, found
}
