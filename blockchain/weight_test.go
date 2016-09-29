package blockchain

// TODO(roasbeef): sig op, weight, etc

/*func TestTxVirtualSize(t *testing.T) {
	tests := []struct {
		in   *wire.MsgTx // Tx to encode
		size int         // Expected virtual size
	}{
		// Transaction with an input which includes witness data, and
		// one output.
		//{multiWitnessTx, 109},
	}
	// TODO(roasbeef): more cases
	// * move this to blockchain

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		serializedSize := GetTxVirtualSize(btcutil.NewTx(test.in))
		if serializedSize != test.size {
			t.Errorf("MsgTx.VirtualSize: #%d got: %d, want: %d", i,
				serializedSize, test.size)
			continue
		}
	}
}*/
