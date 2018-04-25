// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
)

// TestBlockHeader tests the BlockHeader API.
func TestBlockHeader(t *testing.T) {
	nonce64, err := RandomUint64()
	if err != nil {
		t.Errorf("RandomUint64: Error generating nonce: %v", err)
	}
	nonce := uint32(nonce64)

	hash := mainNetGenesisHash
	merkleHash := mainNetGenesisMerkleRoot
	bits := uint32(0x1e0ffff0)
	bh := NewBlockHeader(1, &hash, &merkleHash, bits, nonce)

	// Ensure we get the same data back out.
	if !bh.PrevBlock.IsEqual(&hash) {
		t.Errorf("NewBlockHeader: wrong prev hash - got %v, want %v",
			spew.Sprint(bh.PrevBlock), spew.Sprint(hash))
	}
	if !bh.MerkleRoot.IsEqual(&merkleHash) {
		t.Errorf("NewBlockHeader: wrong merkle root - got %v, want %v",
			spew.Sprint(bh.MerkleRoot), spew.Sprint(merkleHash))
	}
	if bh.Bits != bits {
		t.Errorf("NewBlockHeader: wrong bits - got %v, want %v",
			bh.Bits, bits)
	}
	if bh.Nonce != nonce {
		t.Errorf("NewBlockHeader: wrong nonce - got %v, want %v",
			bh.Nonce, nonce)
	}
}

// TestBlockHeaderWire tests the BlockHeader wire encode and decode for various
// protocol versions.
func TestBlockHeaderWire(t *testing.T) {
	nonce := uint32(1975193600) // 0x1e0f3
	pver := uint32(70001)

	// baseBlockHdr is used in the various tests as a baseline BlockHeader.
	bits := uint32(0x1e0ffff0)
	baseBlockHdr := &BlockHeader{
		Version:    2,
		PrevBlock:  mainNetGenesisHash,
		MerkleRoot: mainNetGenesisMerkleRoot,
		Timestamp:  time.Unix(0x52c283f0, 0), // 2013-12-31 08:44:32 +0000 UTC
		Bits:       bits,
		Nonce:      nonce,
	}

	// baseBlockHdrEncoded is the wire encoded bytes of baseBlockHdr.
	baseBlockHdrEncoded := []byte{
		0x02, 0x00, 0x00, 0x00, // Version 2
		0xb6, 0x8b, 0x8c, 0x41, 0x0d, 0x2e, 0xa4, 0xaf, 
		0xd7, 0x4f, 0xb5, 0x6e, 0x37, 0x0b, 0xfc, 0x1b, 
		0xed, 0xf9, 0x29, 0xe1, 0x45, 0x38, 0x96, 0xc9, 
		0xe7, 0x9d, 0xd1, 0x16, 0x01, 0x1c, 0x9f, 0xff, // PrevBlock
		0xa6, 0x4b, 0xac, 0x07, 0xfe, 0x31, 0x87, 0x7f,
		0x31, 0xd0, 0x32, 0x52, 0x95, 0x3b, 0x3c, 0x32,
		0x39, 0x89, 0x33, 0xaf, 0x7a, 0x72, 0x41, 0x19,
		0xbc, 0x4d, 0x6f, 0xa4, 0xa8, 0x05, 0xe4, 0x35, // MerkleRoot
		0xf0, 0x83, 0xc2, 0x52, // Timestamp
		0xf0, 0xff, 0x0f, 0x1e, // Bits
		0x00, 0x10, 0xbb, 0x75, // Nonce
	}

	tests := []struct {
		in   *BlockHeader    // Data to encode
		out  *BlockHeader    // Expected decoded data
		buf  []byte          // Wire encoding
		pver uint32          // Protocol version for wire encoding
		enc  MessageEncoding // Message encoding variant to use
	}{
		// Latest protocol version.
		{
			baseBlockHdr,
			baseBlockHdr,
			baseBlockHdrEncoded,
			ProtocolVersion,
			BaseEncoding,
		},

		// Protocol version BIP0035Version.
		{
			baseBlockHdr,
			baseBlockHdr,
			baseBlockHdrEncoded,
			BIP0035Version,
			BaseEncoding,
		},

		// Protocol version BIP0031Version.
		{
			baseBlockHdr,
			baseBlockHdr,
			baseBlockHdrEncoded,
			BIP0031Version,
			BaseEncoding,
		},

		// Protocol version NetAddressTimeVersion.
		{
			baseBlockHdr,
			baseBlockHdr,
			baseBlockHdrEncoded,
			NetAddressTimeVersion,
			BaseEncoding,
		},

		// Protocol version MultipleAddressVersion.
		{
			baseBlockHdr,
			baseBlockHdr,
			baseBlockHdrEncoded,
			MultipleAddressVersion,
			BaseEncoding,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode to wire format.
		var buf bytes.Buffer
		err := writeBlockHeader(&buf, test.pver, test.in)
		if err != nil {
			t.Errorf("writeBlockHeader #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("writeBlockHeader #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}

		buf.Reset()
		err = test.in.BtcEncode(&buf, pver, 0)
		if err != nil {
			t.Errorf("BtcEncode #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("BtcEncode #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}

		// Decode the block header from wire format.
		var bh BlockHeader
		rbuf := bytes.NewReader(test.buf)
		err = readBlockHeader(rbuf, test.pver, &bh)
		if err != nil {
			t.Errorf("readBlockHeader #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&bh, test.out) {
			t.Errorf("readBlockHeader #%d\n got: %s want: %s", i,
				spew.Sdump(&bh), spew.Sdump(test.out))
			continue
		}

		rbuf = bytes.NewReader(test.buf)
		err = bh.BtcDecode(rbuf, pver, test.enc)
		if err != nil {
			t.Errorf("BtcDecode #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&bh, test.out) {
			t.Errorf("BtcDecode #%d\n got: %s want: %s", i,
				spew.Sdump(&bh), spew.Sdump(test.out))
			continue
		}
	}
}

// TestBlockHeaderSerialize tests BlockHeader serialize and deserialize.
func TestBlockHeaderSerialize(t *testing.T) {
	nonce := uint32(1975193600) // 0x1e0f3

	// baseBlockHdr is used in the various tests as a baseline BlockHeader.
	bits := uint32(0x1e0ffff0)
	baseBlockHdr := &BlockHeader{
		Version:    2,
		PrevBlock:  mainNetGenesisHash,
		MerkleRoot: mainNetGenesisMerkleRoot,
		Timestamp:  time.Unix(0x52c283f0, 0), // 2013-12-31 08:44:32 +0000 UTC
		Bits:       bits,
		Nonce:      nonce,
	}

	// baseBlockHdrEncoded is the wire encoded bytes of baseBlockHdr.
	baseBlockHdrEncoded := []byte{
		0x02, 0x00, 0x00, 0x00, // Version 2
		0xb6, 0x8b, 0x8c, 0x41, 0x0d, 0x2e, 0xa4, 0xaf, 
		0xd7, 0x4f, 0xb5, 0x6e, 0x37, 0x0b, 0xfc, 0x1b, 
		0xed, 0xf9, 0x29, 0xe1, 0x45, 0x38, 0x96, 0xc9, 
		0xe7, 0x9d, 0xd1, 0x16, 0x01, 0x1c, 0x9f, 0xff, // PrevBlock
		0xa6, 0x4b, 0xac, 0x07, 0xfe, 0x31, 0x87, 0x7f,
		0x31, 0xd0, 0x32, 0x52, 0x95, 0x3b, 0x3c, 0x32,
		0x39, 0x89, 0x33, 0xaf, 0x7a, 0x72, 0x41, 0x19,
		0xbc, 0x4d, 0x6f, 0xa4, 0xa8, 0x05, 0xe4, 0x35, // MerkleRoot
		0xf0, 0x83, 0xc2, 0x52, // Timestamp
		0xf0, 0xff, 0x0f, 0x1e, // Bits
		0x00, 0x10, 0xbb, 0x75, // Nonce
	}

	tests := []struct {
		in  *BlockHeader // Data to encode
		out *BlockHeader // Expected decoded data
		buf []byte       // Serialized data
	}{
		{
			baseBlockHdr,
			baseBlockHdr,
			baseBlockHdrEncoded,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Serialize the block header.
		var buf bytes.Buffer
		err := test.in.Serialize(&buf)
		if err != nil {
			t.Errorf("Serialize #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("Serialize #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}

		// Deserialize the block header.
		var bh BlockHeader
		rbuf := bytes.NewReader(test.buf)
		err = bh.Deserialize(rbuf)
		if err != nil {
			t.Errorf("Deserialize #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&bh, test.out) {
			t.Errorf("Deserialize #%d\n got: %s want: %s", i,
				spew.Sdump(&bh), spew.Sdump(test.out))
			continue
		}
	}
}
