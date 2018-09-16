// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"github.com/wakiyamap/monad/btcec"
	"github.com/wakiyamap/monad/chaincfg/chainhash"
	"github.com/wakiyamap/monad/database"
)

const (
	checkpointWriteThreshol = 20
)

// alert payload's signature check
func CheckSignature(alertKey []byte, serializedPayload []byte, signature []byte) bool {
	pAlertPubKey, err := btcec.ParsePubKey(alertKey, btcec.S256())
	if err != nil {
		return false
	}

	pSignature, err := btcec.ParseSignature(signature, btcec.S256())
	if err != nil {
		return false
	}
	if !pSignature.Verify(chainhash.DoubleHashB(serializedPayload), pAlertPubKey) {
		return false
	}
	return true
}

// Writing the specified checkpoint.
func CmdCheckpoint(height int64, hash string, serverHeight int64, serverHash string, minVer int64) {
	uc := database.GetUserCheckpointDbInstance()
	ucMax := uc.GetMaxCheckpointHeight()
	if height == minVer {
		if height > ucMax && height < serverHeight {
			if height >= ucMax+checkpointWriteThreshol {
				if hash == serverHash {
					uc.Add(height, hash)
				}
			}
			vc := database.GetVolatileCheckpointDbInstance()
			vc.Set(height, hash)
		}
	} else {
		peerLog.Infof("ALERT, MinVer %v does not match %v", minVer, height)
	}
}

// Invalidation of specified alertkey.
func CmdInvalidateKey(key string) {
	ak := database.GetAlertKeyDbInstance()
	ak.Set(key)
}
