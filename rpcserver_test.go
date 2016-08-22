// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpctest"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

func testGetBestBlock(r *rpctest.Harness, t *testing.T) {
	_, prevbestHeight, err := r.Node.GetBestBlock()
	if err != nil {
		t.Fatalf("Call to `getbestblock` failed: %v", err)
	}

	// Create a new block connecting to the current tip.
	generatedBlockHashes, err := r.Node.Generate(1)
	if err != nil {
		t.Fatalf("Unable to generate block: %v", err)
	}

	bestHash, bestHeight, err := r.Node.GetBestBlock()
	if err != nil {
		t.Fatalf("Call to `getbestblock` failed: %v", err)
	}

	// Hash should be the same as the newly submitted block.
	if !bytes.Equal(bestHash[:], generatedBlockHashes[0][:]) {
		t.Fatalf("Block hashes do not match. Returned hash %v, wanted "+
			"hash %v", bestHash, generatedBlockHashes[0][:])
	}

	// Block height should now reflect newest height.
	if bestHeight != prevbestHeight+1 {
		t.Fatalf("Block heights do not match. Got %v, wanted %v",
			bestHeight, prevbestHeight+1)
	}
}

func testGetBlockCount(r *rpctest.Harness, t *testing.T) {
	// Save the current count.
	currentCount, err := r.Node.GetBlockCount()
	if err != nil {
		t.Fatalf("Unable to get block count: %v", err)
	}

	if _, err := r.Node.Generate(1); err != nil {
		t.Fatalf("Unable to generate block: %v", err)
	}

	// Count should have increased by one.
	newCount, err := r.Node.GetBlockCount()
	if err != nil {
		t.Fatalf("Unable to get block count: %v", err)
	}
	if newCount != currentCount+1 {
		t.Fatalf("Block count incorrect. Got %v should be %v",
			newCount, currentCount+1)
	}
}

func testGetBlockHash(r *rpctest.Harness, t *testing.T) {
	// Create a new block connecting to the current tip.
	generatedBlockHashes, err := r.Node.Generate(1)
	if err != nil {
		t.Fatalf("Unable to generate block: %v", err)
	}

	info, err := r.Node.GetInfo()
	if err != nil {
		t.Fatalf("call to getinfo cailed: %v", err)
	}

	blockHash, err := r.Node.GetBlockHash(int64(info.Blocks))
	if err != nil {
		t.Fatalf("Call to `getblockhash` failed: %v", err)
	}

	// Block hashes should match newly created block.
	if !bytes.Equal(generatedBlockHashes[0][:], blockHash[:]) {
		t.Fatalf("Block hashes do not match. Returned hash %v, wanted "+
			"hash %v", blockHash, generatedBlockHashes[0][:])
	}
}

func testMedianTimePastLockTime(r *rpctest.Harness, t *testing.T) {
	// We'd like to test the proper adherance of the BIP 113 rule
	// constraint which requires all transaction finality tests to use the
	// MTP of the last 11 blocks, rather than the timestamp of the block
	// which includes them.

	// First, create a series of timestamps we'll use. Each block has a
	// timestamp exactly 10 minutes after the previous block.
	const medianTimeBlocks = 11
	timeStamps := make([]time.Time, medianTimeBlocks)
	timeStamps[0] = time.Now().Add(time.Minute)
	for i := 1; i < medianTimeBlocks; i++ {
		timeStamps[i] = timeStamps[i-1].Add(time.Minute * 10)
	}

	// With the stamps created, generate a series of empty blocks for each
	// of the timestamps.
	blocks := make([]*btcutil.Block, 0, len(timeStamps))
	for _, timeStamp := range timeStamps {
		block, err := r.GenerateAndSubmitBlock(nil, -1, timeStamp)
		if err != nil {
			t.Fatalf("unable to generate block: %v", err)
		}
		blocks = append(blocks, block)
	}

	// The current MTP should be the timestamp of 6th block generated.
	currentMTP := blocks[5].MsgBlock().Header.Timestamp

	// Fetch a fresh address from the harness, we'll use this within the
	// transaction we create below shortly.
	addr, err := r.NewAddress()
	if err != nil {
		t.Fatalf("unable to generate address: %v", err)
	}
	addrScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("unable to generate addr script: %v", err)
	}

	// Create a fresh key, then send some coins to an address spendable by
	// that key.
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Fatalf("unable to generate key: %v", err)
	}
	a, err := btcutil.NewAddressPubKey(key.PubKey().SerializeCompressed(), r.ActiveNet)
	if err != nil {
		t.Fatalf("unable to create address: %v", err)
	}
	selfAddrScript, err := txscript.PayToAddrScript(a.AddressPubKeyHash())
	if err != nil {
		t.Fatalf("unable to generate addr script: %v", err)
	}
	output := &wire.TxOut{PkScript: selfAddrScript, Value: 1e8}
	fundTx, err := r.CreateTransaction([]*wire.TxOut{output}, 10)
	if err != nil {
		t.Fatalf("unable to send outputs: %v", err)
	}
	if _, err := r.Node.SendRawTransaction(fundTx, true); err != nil {
		t.Fatalf("Unable to broadcast transaction: %v", err)
	}

	// Locate the output index of the coins spendable by the key we
	// generated above. We'll need this to craft a new transaction shortly
	// below.
	var outputIndex uint32
	if bytes.Equal(fundTx.TxOut[0].PkScript, selfAddrScript) {
		outputIndex = 0
	} else {
		outputIndex = 1
	}

	// Now create a transaction with a lock time which is "final" according
	// to the latest block, but not according to the current median time
	// past.
	tx := wire.NewMsgTx()
	tx.LockTime = uint32(blocks[len(blocks)-1].MsgBlock().Header.Timestamp.Unix())
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  fundTx.TxHash(),
			Index: outputIndex,
		},
	})
	tx.AddTxOut(&wire.TxOut{
		PkScript: addrScript,
		Value:    btcutil.SatoshiPerBitcoin - 1000,
	})
	sigScript, err := txscript.SignatureScript(tx, 0, selfAddrScript,
		txscript.SigHashAll, key, true)
	if err != nil {
		t.Fatalf("unable to generate sig: %v", err)
	}
	tx.TxIn[0].SignatureScript = sigScript

	// This transaction should be rejected. Additionally, the exact error
	// should be the rejection of a non-final transaction.
	_, err = r.Node.SendRawTransaction(tx, true)
	if err == nil {
		t.Fatalf("transaction accepted, but should be non-final")
	} else if !strings.Contains(err.Error(), "not finalized") {
		t.Fatalf("transacvtion should be rejected due to being "+
			"non-final, instead :%v", err)
	}

	// Modify the transaction to have a valid lock-time considered "final"
	// according to the current MTP.
	tx.LockTime = uint32(currentMTP.Unix()) - 30
	sigScript, err = txscript.SignatureScript(tx, 0, selfAddrScript,
		txscript.SigHashAll, key, true)
	if err != nil {
		t.Fatalf("unable to generate sig: %v", err)
	}
	tx.TxIn[0].SignatureScript = sigScript

	// This transaction should now be accepted to the mempool.
	txid, err := r.Node.SendRawTransaction(tx, true)
	if err != nil {
		t.Fatalf("unable to send transaction: %v", err)
	}

	// Mine a single block, the transaction within the mempool should now
	// be within this block.
	blockHash, err := r.Node.Generate(1)
	if err != nil {
		t.Fatalf("unable to generate block: %v", err)
	}
	block, err := r.Node.GetBlock(blockHash[0])
	if err != nil {
		t.Fatalf("unable to obtain block: %v", err)
	}
	for _, txn := range block.Transactions() {
		if txn.Hash().IsEqual(txid) {
			return
		}
	}

	// If we've reeached this point, then our time-locked transaction
	// wasn't selected to be included within this block.
	t.Fatalf("lock-time transaction not included within block")
}

var rpcTestCases = []rpctest.HarnessTestCase{
	testGetBestBlock,
	testGetBlockCount,
	testGetBlockHash,
	testMedianTimePastLockTime,
}

var primaryHarness *rpctest.Harness

func TestMain(m *testing.M) {
	var err error

	// In order to properly test scenarios on as if we were on mainnet,
	// ensure that non-standard transactions aren't accepted into the
	// mempool or relayed.
	btcdCfg := []string{"--rejectnonstd"}
	primaryHarness, err = rpctest.New(&chaincfg.SimNetParams, nil, btcdCfg)
	if err != nil {
		fmt.Println("unable to create primary harness: ", err)
		os.Exit(1)
	}

	// Initialize the primary mining node with a chain of length 125,
	// providing 25 mature coinbases to allow spending from for testing
	// purposes.
	if err := primaryHarness.SetUp(true, 25); err != nil {
		fmt.Println("unable to setup test chain: ", err)
		os.Exit(1)
	}

	exitCode := m.Run()

	// Clean up any active harnesses that are still currently running.This
	// includes removing all temporary directories, and shutting down any
	// created processes.
	if err := rpctest.TearDownAll(); err != nil {
		fmt.Println("unable to tear down all harnesses: ", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func TestRpcServer(t *testing.T) {
	var currentTestNum int
	defer func() {
		// If one of the integration tests caused a panic within the main
		// goroutine, then tear down all the harnesses in order to avoid
		// any leaked btcd processes.
		if r := recover(); r != nil {
			fmt.Println("recovering from test panic: ", r)
			if err := rpctest.TearDownAll(); err != nil {
				fmt.Println("unable to tear down all harnesses: ", err)
			}
			t.Fatalf("test #%v panicked: %s", currentTestNum, debug.Stack())
		}
	}()

	for _, testCase := range rpcTestCases {
		testCase(primaryHarness, t)

		currentTestNum++
	}
}
