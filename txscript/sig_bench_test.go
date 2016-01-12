package txscript

import (
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/wire"
)

var sighash = wire.ShaHash([32]byte{
	0xc9, 0x97, 0xa5, 0xe5,
	0x6e, 0x10, 0x41, 0x02,
	0xfa, 0x20, 0x9c, 0x6a,
	0x85, 0x2d, 0xd9, 0x06,
	0x60, 0xa2, 0x0b, 0x2d,
	0x9c, 0x35, 0x24, 0x23,
	0xed, 0xce, 0x25, 0x85,
	0x7f, 0xcd, 0x37, 0x04,
})

var privKey, pubKey = btcec.PrivKeyFromBytes(
	btcec.S256(),
	[]byte{
		0x60, 0xa2, 0x0b, 0x2d,
		0xfa, 0x20, 0x9c, 0x6a,
		0xc9, 0x97, 0xa5, 0xe5,
		0x7f, 0xcd, 0x37, 0x04,
		0x6e, 0x10, 0x41, 0x02,
		0x9c, 0x35, 0x24, 0x23,
		0xed, 0xce, 0x25, 0x85,
		0x85, 0x2d, 0xd9, 0x06,
	})

var sig, _ = privKey.Sign(sighash[:])

func BenchmarkSigCacheOldExists(b *testing.B) {
	sigcache := NewOldSigCache(10)

	sigcache.Add(sighash, sig, pubKey)
	for i := 0; i < b.N; i++ {
		_ = sigcache.Exists(sighash, sig, pubKey)
	}
}

func BenchmarkSigCacheOldStringExists(b *testing.B) {
	sigcache := NewOldSigCacheString(10)

	sigcache.Add(sighash, sig, pubKey)
	for i := 0; i < b.N; i++ {
		_ = sigcache.Exists(sighash, sig, pubKey)
	}
}

func BenchmarkSigCacheNewExists(b *testing.B) {
	sigcache, err := NewSigCache(10)
	if err != nil {
		b.Fatalf("unable to create sigcache: %v", err)
	}

	sigcache.Add(sighash, sig, pubKey)
	for i := 0; i < b.N; i++ {
		_ = sigcache.Exists(sighash, sig, pubKey)
	}
}

func BenchmarkSigCacheNewStringExists(b *testing.B) {
	sigcache, err := NewStringSigCache(10)
	if err != nil {
		b.Fatalf("unable to create sigcache: %v", err)
	}

	sigcache.Add(sighash, sig, pubKey)
	for i := 0; i < b.N; i++ {
		_ = sigcache.Exists(sighash, sig, pubKey)
	}
}

func BenchmarkSigCacheOldAdd(b *testing.B) {
	sigcache := NewOldSigCache(10)

	for i := 0; i < b.N; i++ {
		sigcache.Add(sighash, sig, pubKey)
	}
}

func BenchmarkSigCacheOldStringAdd(b *testing.B) {
	sigcache := NewOldSigCacheString(10)

	for i := 0; i < b.N; i++ {
		sigcache.Add(sighash, sig, pubKey)
	}
}

func BenchmarkSigCacheNewAdd(b *testing.B) {
	sigcache, err := NewSigCache(10)
	if err != nil {
		b.Fatalf("unable to create sigcache: %v", err)
	}

	for i := 0; i < b.N; i++ {
		sigcache.Add(sighash, sig, pubKey)
	}
}

func BenchmarkSigCacheNewStringAdd(b *testing.B) {
	sigcache, err := NewStringSigCache(10)
	if err != nil {
		b.Fatalf("unable to create sigcache: %v", err)
	}

	for i := 0; i < b.N; i++ {
		sigcache.Add(sighash, sig, pubKey)
	}
}
