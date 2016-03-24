// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import "github.com/btcsuite/btcd/wire"

// HashCache.  Get it???  For Segwit.  Per-tx hashes instead of per-txin.
type HashCache struct {
	HashPrevOuts wire.ShaHash
	HashSequence wire.ShaHash
	HashOutputs  wire.ShaHash
}

// TODO(roasbeef): make into map[txid]cachedHashes

// CalcHashCache calculates the hashes which are used to make sighashes to sign.
func CalcHashCache(tx *wire.MsgTx, inIndex int, hType SigHashType) HashCache {
	var hc HashCache
	hc.HashPrevOuts = calcHashPrevOuts(tx, hType)
	hc.HashSequence = calcHashSequence(tx, hType)
	hc.HashOutputs = calcHashOutputs(tx, inIndex, hType)

	return hc
}
