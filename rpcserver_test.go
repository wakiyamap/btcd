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

	"github.com/btcsuite/btcd/blockchain"
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

func testSequenceLocksValidation(r *rpctest.Harness, t *testing.T) {
	// We'd like the test proper evaluation and validation of the BIP 68
	// (sequence locks) rule-set which adds input-age based relative lock
	// times.
	t.Log("Running tests for BIP 68")

	harnessAddr, err := r.NewAddress()
	if err != nil {
		t.Fatalf("unable to generate new address: %v", err)
	}
	harnessScript, err := txscript.PayToAddrScript(harnessAddr)
	if err != nil {
		t.Fatalf("unable to generate addr script: %v", err)
	}

	// Generate a fresh key, then send create a series of outputs spendable
	// by the key.
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Fatalf("unable to generate key: %v", err)
	}
	a, err := btcutil.NewAddressPubKey(key.PubKey().SerializeCompressed(), r.ActiveNet)
	if err != nil {
		t.Fatalf("unable to create address: %v", err)
	}
	testPkScript, err := txscript.PayToAddrScript(a.AddressPubKeyHash())
	if err != nil {
		t.Fatalf("unable to generate addr script: %v", err)
	}

	// Knowing the number of outputs needed for the tests below, create
	const numTests = 6
	var spendableInputs [numTests]wire.OutPoint
	for i := 0; i < numTests; i++ {
		output := &wire.TxOut{
			PkScript: testPkScript,
			Value:    btcutil.SatoshiPerBitcoin,
		}
		tx, err := r.CreateTransaction([]*wire.TxOut{output}, 10)
		if err != nil {
			t.Fatalf("unable to send transaction: %v", err)
		}

		if _, err := r.Node.SendRawTransaction(tx, true); err != nil {
			t.Fatalf("unable to broadcast transaction: %v", err)
		}

		var outputIndex uint32
		if bytes.Equal(tx.TxOut[0].PkScript, testPkScript) {
			outputIndex = 0
		} else {
			outputIndex = 1
		}

		spendableInputs[i] = wire.OutPoint{
			Hash:  tx.TxHash(),
			Index: outputIndex,
		}
	}

	// Mine a single block including all the transactions generated above.
	if _, err := r.Node.Generate(1); err != nil {
		t.Fatalf("unable to generate block: %v", err)
	}

	// Now mine 5 additional blocks giving the inputs generated above a age
	// of 5. Space out each block 10 minutes after the previous block.
	prevBlockHash, err := r.Node.GetBestBlockHash()
	if err != nil {
		t.Fatalf("unable to get prior block hash: %v", err)
	}
	prevBlock, err := r.Node.GetBlock(prevBlockHash)
	if err != nil {
		t.Fatalf("unable to get block: %v", err)
	}
	for i := 0; i < 5; i++ {
		timeStamp := prevBlock.MsgBlock().Header.Timestamp.Add(time.Minute * 10)
		prevBlock, err = r.GenerateAndSubmitBlock(nil, -1, timeStamp)
		if err != nil {
			t.Fatalf("unable to generate block: %v", err)
		}
	}

	// A helper function to create fully signed transactions in-line during
	// the array initialization below.
	var inputIndex uint32
	makeTxCase := func(sequenceNum uint32, txVersion int32) *wire.MsgTx {
		tx := wire.NewMsgTx()
		tx.Version = txVersion
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: spendableInputs[inputIndex],
			Sequence:         sequenceNum,
		})
		tx.AddTxOut(&wire.TxOut{
			PkScript: harnessScript,
			Value:    btcutil.SatoshiPerBitcoin - 1000,
		})

		sigScript, err := txscript.SignatureScript(tx, 0, testPkScript,
			txscript.SigHashAll, key, true)
		if err != nil {
			t.Fatalf("unable to generate sig: %v", err)
		}
		tx.TxIn[0].SignatureScript = sigScript

		inputIndex++
		return tx
	}

	tests := [numTests]struct {
		tx *wire.MsgTx

		accept bool
	}{
		// A valid transaction with a single input a sequence number
		// creating a 100 block relative time-lock. This transaction
		// should be accepted as its version number is 1, and only tx
		// of version > 2 will trigger the sequence lock behavior.
		{
			tx:     makeTxCase(blockchain.LockTimeToSequence(false, 100), 1),
			accept: true,
		},
		// A transaction of version 2 spending a single input. The
		// input has a relative time-lock of 100 blocks, but the
		// disable bit it set. The transaction should be accepted as a
		// result.
		{
			tx: makeTxCase(
				blockchain.LockTimeToSequence(false, 100)|blockchain.RelativeLockTimeDisabled,
				2,
			),
			accept: true,
		},
		// A v2 transaction with a single input having a 2 lock
		// relative time lock. The referenced input is 5 blocks old so
		// the transaction should be accepted.
		{
			tx:     makeTxCase(blockchain.LockTimeToSequence(false, 2), 2),
			accept: true,
		},
		// A v2 transaction whose input has a 1000 relative time lock.
		// This should be rejected as the input's age is only 5 blocks.
		{
			tx:     makeTxCase(blockchain.LockTimeToSequence(false, 1000), 2),
			accept: false,
		},
		// A v2 transaction with a single input having a 512,000 second
		// relative time-lock. This transaction should be rejected as 6
		// days worth of blocks haven't yet been mined. The referenced
		// input doesn't have sufficient age.
		{
			tx:     makeTxCase(blockchain.LockTimeToSequence(true, 512000), 2),
			accept: false,
		},
		// A v2 transaction whose single input has a 512 second
		// relative time-lock. This transaction should be accepted as
		// finalized.
		{
			tx:     makeTxCase(blockchain.LockTimeToSequence(true, 512), 2),
			accept: true,
		},
	}

	for i, test := range tests {
		txid, err := r.Node.SendRawTransaction(test.tx, true)
		switch {
		// Test case passes, nothing further to report.
		case test.accept && err == nil:

		// Transaction should have been accepted but we have a non-nil
		// error.
		case test.accept && err != nil:
			t.Fatalf("test #%d, transaction should be accepted, "+
				"but was rejected: %v", i, err)

		// Transaction should have been rejected, but it was accepted.
		case !test.accept && err == nil:
			t.Fatalf("test #%d, transaction should be rejected, "+
				"but was accepted", i)

		// Transaction was rejected as wanted, but ensure the proper
		// error message was generated.
		case !test.accept && err != nil:
			if !strings.Contains(err.Error(), "sequence lock") {
				t.Fatalf("test #%d, transaction should be rejected "+
					"due to inactive sequence locks, "+
					"instead: %v", i, err)
			}
		}

		// If the transaction should be rejected, manually mine a block
		// with the non-final transaction. It shouold be rejected.
		if !test.accept {
			txns := []*btcutil.Tx{btcutil.NewTx(test.tx)}
			_, err := r.GenerateAndSubmitBlock(txns, -1, time.Time{})
			if err == nil {
				t.Fatalf("test #%d, invalid block accepted", i)
			} else if !strings.Contains(err.Error(), "sequence lock") {
				t.Fatalf("test #%d, block should be rejected "+
					"due to inactive sequence locks, "+
					"instead: %v", i, err)
			}

			continue
		}

		// Generate a block, the transaction should be included within
		// the newly mined block.
		blockHashes, err := r.Node.Generate(1)
		if err != nil {
			t.Fatalf("unable to mine block: %v", err)
		}
		block, err := r.Node.GetBlock(blockHashes[0])
		if err != nil {
			t.Fatalf("unable to get block: %v", err)
		}
		if len(block.Transactions()) < 2 {
			t.Fatalf("target transaction was not mined")
		}

		var found bool
		for _, txn := range block.Transactions() {
			if txn.Hash().IsEqual(txid) {
				found = true
				break
			}
		}

		if !found {
			// If we've reached this point, then the tranasaction wasn't
			// included the block but should have been.
			t.Fatalf("test #%d, transation was not included in block", i)
		}
	}
}

var rpcTestCases = []rpctest.HarnessTestCase{
	testGetBestBlock,
	testGetBlockCount,
	testGetBlockHash,
	testMedianTimePastLockTime,
	testSequenceLocksValidation,
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
