package database

import (
	"path/filepath"
	"sync"
	"time"
	"flag"

	"github.com/btcsuite/goleveldb/leveldb"
	"github.com/wakiyamap/monad/chaincfg"
)

const (
	// volatileCheckpointDbNamePrefix is the prefix for the monad volatilecheckpoint database.
	volatileCheckpointDbNamePrefix = "volatilecheckpoints"
)

type VolatileCheckpoint struct {
	Vcdb *leveldb.DB
}

var vinstance *VolatileCheckpoint
var vonce sync.Once

func (vc *VolatileCheckpoint) OpenDB() error {
	if vc.Vcdb != nil {
		return nil
	}

	var err error
	dbpath := GetVolatileCheckpointDbPath()
	vc.Vcdb, err = leveldb.OpenFile(dbpath, nil)
	return err
}

func (vc *VolatileCheckpoint) CloseDB() {
	if vc.Vcdb == nil {
		return
	}
	vc.Vcdb.Close()
	vc.Vcdb = nil
}

func GetVolatileCheckpointDbInstance() *VolatileCheckpoint {
	vonce.Do(func() {
		time.Sleep(1 * time.Second)
		vinstance = &VolatileCheckpoint{nil}
	})
	return vinstance
}

func GetVolatileCheckpointDbPath() (dbPath string) {
	flag.Parse()
	// There is a possibility that simnet and regtest will not work for this sentence.
	if *testnet {
		activeNetParams = &chaincfg.TestNet4Params
	}
	dbName := volatileCheckpointDbNamePrefix + "_" + defaultDbType
	dbPath = filepath.Join(defaultDataDir, netName(activeNetParams), dbName)

	return dbPath
}
