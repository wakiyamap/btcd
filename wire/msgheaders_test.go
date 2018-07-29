// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"io"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

// TestHeaders tests the MsgHeaders API.
func TestHeaders(t *testing.T) {
	pver := uint32(60002)

	// Ensure the command is expected value.
	wantCmd := "headers"
	msg := NewMsgHeaders()
	if cmd := msg.Command(); cmd != wantCmd {
		t.Errorf("NewMsgHeaders: wrong command - got %v want %v",
			cmd, wantCmd)
	}

	// Ensure max payload is expected value for latest protocol version.
	// Num headers (varInt) + max allowed headers (header length + 1 byte
	// for the number of transactions which is always 0).
	wantPayload := uint32(162009)
	maxPayload := msg.MaxPayloadLength(pver)
	if maxPayload != wantPayload {
		t.Errorf("MaxPayloadLength: wrong max payload length for "+
			"protocol version %d - got %v, want %v", pver,
			maxPayload, wantPayload)
	}

	// Ensure headers are added properly.
	bh := &blockOne.Header
	msg.AddBlockHeader(bh)
	if !reflect.DeepEqual(msg.Headers[0], bh) {
		t.Errorf("AddHeader: wrong header - got %v, want %v",
			spew.Sdump(msg.Headers),
			spew.Sdump(bh))
	}

	// Ensure adding more than the max allowed headers per message returns
	// error.
	var err error
	for i := 0; i < MaxBlockHeadersPerMsg+1; i++ {
		err = msg.AddBlockHeader(bh)
	}
	if reflect.TypeOf(err) != reflect.TypeOf(&MessageError{}) {
		t.Errorf("AddBlockHeader: expected error on too many headers " +
			"not received")
	}
}

// TestHeadersWire tests the MsgHeaders wire encode and decode for various
// numbers of headers and protocol versions.
func TestHeadersWire(t *testing.T) {
	hash := mainNetGenesisHash
	merkleHash := blockOne.Header.MerkleRoot
	bits := uint32(0x1e0ffff0)
	nonce := uint32(0x75bb1000)
	bh := NewBlockHeader(1, &hash, &merkleHash, bits, nonce)
	bh.Version = blockOne.Header.Version
	bh.Timestamp = blockOne.Header.Timestamp

	// Empty headers message.
	noHeaders := NewMsgHeaders()
	noHeadersEncoded := []byte{
		0x00, // Varint for number of headers
	}

	// Headers message with one header.
	oneHeader := NewMsgHeaders()
	oneHeader.AddBlockHeader(bh)
	oneHeaderEncoded := []byte{
		0x01,                   // VarInt for number of headers.
		0x02, 0x00, 0x00, 0x00, // Version 2
		0xb6, 0x8b, 0x8c, 0x41, 0x0d, 0x2e, 0xa4, 0xaf,
		0xd7, 0x4f, 0xb5, 0x6e, 0x37, 0x0b, 0xfc, 0x1b,
		0xed, 0xf9, 0x29, 0xe1, 0x45, 0x38, 0x96, 0xc9,
		0xe7, 0x9d, 0xd1, 0x16, 0x01, 0x1c, 0x9f, 0xff, // PrevBlock
		0xb9, 0xc3, 0x9c, 0x20, 0xb4, 0xba, 0xf3, 0xb8,
		0x4c, 0xd7, 0x23, 0x49, 0x30, 0x08, 0x33, 0x08,
		0x6d, 0x68, 0x6d, 0xb1, 0x42, 0xc5, 0x1b, 0x26,
		0xa1, 0x96, 0xcd, 0xab, 0xbe, 0x7a, 0x06, 0x10, // MerkleRoot
		0xd4, 0x5a, 0xc3, 0x52, // Timestamp
		0xf0, 0xff, 0x0f, 0x1e, // Bits
		0x00, 0x10, 0xbb, 0x75, // Nonce
		0x00, // TxnCount (0 for headers message)
	}

	tests := []struct {
		in   *MsgHeaders     // Message to encode
		out  *MsgHeaders     // Expected decoded message
		buf  []byte          // Wire encoding
		pver uint32          // Protocol version for wire encoding
		enc  MessageEncoding // Message encoding format
	}{
		// Latest protocol version with no headers.
		{
			noHeaders,
			noHeaders,
			noHeadersEncoded,
			ProtocolVersion,
			BaseEncoding,
		},

		// Latest protocol version with one header.
		{
			oneHeader,
			oneHeader,
			oneHeaderEncoded,
			ProtocolVersion,
			BaseEncoding,
		},

		// Protocol version BIP0035Version with no headers.
		{
			noHeaders,
			noHeaders,
			noHeadersEncoded,
			BIP0035Version,
			BaseEncoding,
		},

		// Protocol version BIP0035Version with one header.
		{
			oneHeader,
			oneHeader,
			oneHeaderEncoded,
			BIP0035Version,
			BaseEncoding,
		},

		// Protocol version BIP0031Version with no headers.
		{
			noHeaders,
			noHeaders,
			noHeadersEncoded,
			BIP0031Version,
			BaseEncoding,
		},

		// Protocol version BIP0031Version with one header.
		{
			oneHeader,
			oneHeader,
			oneHeaderEncoded,
			BIP0031Version,
			BaseEncoding,
		},
		// Protocol version NetAddressTimeVersion with no headers.
		{
			noHeaders,
			noHeaders,
			noHeadersEncoded,
			NetAddressTimeVersion,
			BaseEncoding,
		},

		// Protocol version NetAddressTimeVersion with one header.
		{
			oneHeader,
			oneHeader,
			oneHeaderEncoded,
			NetAddressTimeVersion,
			BaseEncoding,
		},

		// Protocol version MultipleAddressVersion with no headers.
		{
			noHeaders,
			noHeaders,
			noHeadersEncoded,
			MultipleAddressVersion,
			BaseEncoding,
		},

		// Protocol version MultipleAddressVersion with one header.
		{
			oneHeader,
			oneHeader,
			oneHeaderEncoded,
			MultipleAddressVersion,
			BaseEncoding,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode the message to wire format.
		var buf bytes.Buffer
		err := test.in.BtcEncode(&buf, test.pver, test.enc)
		if err != nil {
			t.Errorf("BtcEncode #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("BtcEncode #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}

		// Decode the message from wire format.
		var msg MsgHeaders
		rbuf := bytes.NewReader(test.buf)
		err = msg.BtcDecode(rbuf, test.pver, test.enc)
		if err != nil {
			t.Errorf("BtcDecode #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(&msg, test.out) {
			t.Errorf("BtcDecode #%d\n got: %s want: %s", i,
				spew.Sdump(&msg), spew.Sdump(test.out))
			continue
		}
	}
}

// TestHeadersWireErrors performs negative tests against wire encode and decode
// of MsgHeaders to confirm error paths work correctly.
func TestHeadersWireErrors(t *testing.T) {
	pver := ProtocolVersion
	wireErr := &MessageError{}

	hash := mainNetGenesisHash
	merkleHash := blockOne.Header.MerkleRoot
	bits := uint32(0x1e0ffff0)
	nonce := uint32(0x75bb1000)
	bh := NewBlockHeader(1, &hash, &merkleHash, bits, nonce)
	bh.Version = blockOne.Header.Version
	bh.Timestamp = blockOne.Header.Timestamp

	// Headers message with one header.
	oneHeader := NewMsgHeaders()
	oneHeader.AddBlockHeader(bh)
	oneHeaderEncoded := []byte{
		0x01,                   // VarInt for number of headers.
		0x02, 0x00, 0x00, 0x00, // Version 2
		0xb6, 0x8b, 0x8c, 0x41, 0x0d, 0x2e, 0xa4, 0xaf,
		0xd7, 0x4f, 0xb5, 0x6e, 0x37, 0x0b, 0xfc, 0x1b,
		0xed, 0xf9, 0x29, 0xe1, 0x45, 0x38, 0x96, 0xc9,
		0xe7, 0x9d, 0xd1, 0x16, 0x01, 0x1c, 0x9f, 0xff, // PrevBlock
		0xb9, 0xc3, 0x9c, 0x20, 0xb4, 0xba, 0xf3, 0xb8,
		0x4c, 0xd7, 0x23, 0x49, 0x30, 0x08, 0x33, 0x08,
		0x6d, 0x68, 0x6d, 0xb1, 0x42, 0xc5, 0x1b, 0x26,
		0xa1, 0x96, 0xcd, 0xab, 0xbe, 0x7a, 0x06, 0x10, // MerkleRoot
		0xd4, 0x5a, 0xc3, 0x52, // Timestamp
		0xf0, 0xff, 0x0f, 0x1e, // Bits
		0x00, 0x10, 0xbb, 0x75, // Nonce
		0x00, // TxnCount (0 for headers message)
	}

	// Message that forces an error by having more than the max allowed
	// headers.
	maxHeaders := NewMsgHeaders()
	for i := 0; i < MaxBlockHeadersPerMsg; i++ {
		maxHeaders.AddBlockHeader(bh)
	}
	maxHeaders.Headers = append(maxHeaders.Headers, bh)
	maxHeadersEncoded := []byte{
		0xfd, 0xd1, 0x07, // Varint for number of addresses (2001)7D1
	}

	// Intentionally invalid block header that has a transaction count used
	// to force errors.
	bhTrans := NewBlockHeader(1, &hash, &merkleHash, bits, nonce)
	bhTrans.Version = blockOne.Header.Version
	bhTrans.Timestamp = blockOne.Header.Timestamp

	transHeader := NewMsgHeaders()
	transHeader.AddBlockHeader(bhTrans)
	transHeaderEncoded := []byte{
		0x01,                   // VarInt for number of headers.
		0x02, 0x00, 0x00, 0x00, // Version 2
		0xb6, 0x8b, 0x8c, 0x41, 0x0d, 0x2e, 0xa4, 0xaf,
		0xd7, 0x4f, 0xb5, 0x6e, 0x37, 0x0b, 0xfc, 0x1b,
		0xed, 0xf9, 0x29, 0xe1, 0x45, 0x38, 0x96, 0xc9,
		0xe7, 0x9d, 0xd1, 0x16, 0x01, 0x1c, 0x9f, 0xff, // PrevBlock
		0xb9, 0xc3, 0x9c, 0x20, 0xb4, 0xba, 0xf3, 0xb8,
		0x4c, 0xd7, 0x23, 0x49, 0x30, 0x08, 0x33, 0x08,
		0x6d, 0x68, 0x6d, 0xb1, 0x42, 0xc5, 0x1b, 0x26,
		0xa1, 0x96, 0xcd, 0xab, 0xbe, 0x7a, 0x06, 0x10, // MerkleRoot
		0xd4, 0x5a, 0xc3, 0x52, // Timestamp
		0xf0, 0xff, 0x0f, 0x1e, // Bits
		0x00, 0x10, 0xbb, 0x75, // Nonce
		0x01, // TxnCount (should be 0 for headers message, but 1 to force error)
	}

	tests := []struct {
		in       *MsgHeaders     // Value to encode
		buf      []byte          // Wire encoding
		pver     uint32          // Protocol version for wire encoding
		enc      MessageEncoding // Message encoding format
		max      int             // Max size of fixed buffer to induce errors
		writeErr error           // Expected write error
		readErr  error           // Expected read error
	}{
		// Latest protocol version with intentional read/write errors.
		// Force error in header count.
		{oneHeader, oneHeaderEncoded, pver, BaseEncoding, 0, io.ErrShortWrite, io.EOF},
		// Force error in block header.
		{oneHeader, oneHeaderEncoded, pver, BaseEncoding, 5, io.ErrShortWrite, io.EOF},
		// Force error with greater than max headers.
		{maxHeaders, maxHeadersEncoded, pver, BaseEncoding, 3, wireErr, wireErr},
		// Force error with number of transactions.
		{transHeader, transHeaderEncoded, pver, BaseEncoding, 81, io.ErrShortWrite, io.EOF},
		// Force error with included transactions.
		{transHeader, transHeaderEncoded, pver, BaseEncoding, len(transHeaderEncoded), nil, wireErr},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Encode to wire format.
		w := newFixedWriter(test.max)
		err := test.in.BtcEncode(w, test.pver, test.enc)
		if reflect.TypeOf(err) != reflect.TypeOf(test.writeErr) {
			t.Errorf("BtcEncode #%d wrong error got: %v, want: %v",
				i, err, test.writeErr)
			continue
		}

		// For errors which are not of type MessageError, check them for
		// equality.
		if _, ok := err.(*MessageError); !ok {
			if err != test.writeErr {
				t.Errorf("BtcEncode #%d wrong error got: %v, "+
					"want: %v", i, err, test.writeErr)
				continue
			}
		}

		// Decode from wire format.
		var msg MsgHeaders
		r := newFixedReader(test.max, test.buf)
		err = msg.BtcDecode(r, test.pver, test.enc)
		if reflect.TypeOf(err) != reflect.TypeOf(test.readErr) {
			t.Errorf("BtcDecode #%d wrong error got: %v, want: %v",
				i, err, test.readErr)
			continue
		}

		// For errors which are not of type MessageError, check them for
		// equality.
		if _, ok := err.(*MessageError); !ok {
			if err != test.readErr {
				t.Errorf("BtcDecode #%d wrong error got: %v, "+
					"want: %v", i, err, test.readErr)
				continue
			}
		}

	}
}
