// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"
)

// MsgSendHeaders implements the Message interface and represents a bitcoin
// havewitness message.  I don't know what it does.  Maybe nothing.
type MsgHaveWitness struct{}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgHaveWitness) BtcDecode(r io.Reader, pver uint32) error {
	if pver < SendHeadersVersion {
		str := fmt.Sprintf("havewitness message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgHaveWitness.BtcDecode", str)
	}

	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgHaveWitness) BtcEncode(w io.Writer, pver uint32) error {
	if pver < SendHeadersVersion {
		str := fmt.Sprintf("havewitness message invalid for protocol "+
			"version %d", pver)
		return messageError("MsgHaveWitness.BtcEncode", str)
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgHaveWitness) Command() string {
	return CmdHaveWitness
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgHaveWitness) MaxPayloadLength(pver uint32) uint32 {
	return 0
}

// NewMsgSendHeaders returns a new bitcoin sendheaders message that conforms to
// the Message interface.  See MsgSendHeaders for details.
func NewMsgHaveWitness() *MsgHaveWitness {
	return &MsgHaveWitness{}
}
