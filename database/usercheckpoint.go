package database

import (
	"path/filepath"
	"sync"
	"time"
	"flag"

	"github.com/btcsuite/goleveldb/leveldb"
	"github.com/wakiyamap/monad/chaincfg"
	"github.com/wakiyamap/monad/wire"
	"github.com/wakiyamap/monautil"
)

const (
	// userCheckpointDbNamePrefix is the prefix for the monad block database.
	userCheckpointDbNamePrefix = "usercheckpoints"
	defaultDbType              = "leveldb"
)

var (
	monadHomeDir    = monautil.AppDataDir("monad", false)
	defaultDataDir  = filepath.Join(monadHomeDir, "data")
	activeNetParams = &chaincfg.MainNetParams
	testnet         = flag.Bool("testnet", false, "operate on the testnet Bitcoin network")
)

type UserCheckpoint struct {
	Ucdb *leveldb.DB
}

var instance *UserCheckpoint
var once sync.Once

func (uc *UserCheckpoint) OpenDB() error {
	if uc.Ucdb != nil {
		return nil
	}

	var err error
	dbpath := GetCheckpointDbPath()
	uc.Ucdb, err = leveldb.OpenFile(dbpath, nil)
	return err
}

func (uc *UserCheckpoint) CloseDB() {
	if uc.Ucdb == nil {
		return
	}
	uc.Ucdb.Close()
	uc.Ucdb = nil
}

func GetInstance() *UserCheckpoint {
	once.Do(func() {
		time.Sleep(1 * time.Second)
		instance = &UserCheckpoint{nil}
	})
	return instance
}

// netName returns the name used when referring to a bitcoin network.  At the
// time of writing, monad currently places blocks for testnet version 3 in the
// data and log directory "testnet", which does not match the Name field of the
// chaincfg parameters.  This function can be used to override this directory name
// as "testnet" when the passed active network matches wire.TestNet4.
//
// A proper upgrade to move the data and log directories for this network to
// "testnet4" is planned for the future, at which point this function can be
// removed and the network parameter's name used instead.
func netName(chainParams *chaincfg.Params) string {
	switch chainParams.Net {
	case wire.TestNet4:
		return "testnet"
	default:
		return chainParams.Name
	}
}

func GetCheckpointDbPath() (dbPath string) {
	flag.Parse()
	if *testnet {
		activeNetParams = &chaincfg.TestNet4Params
	}
	dbName := userCheckpointDbNamePrefix + "_" + defaultDbType
	dbPath = filepath.Join(defaultDataDir, netName(activeNetParams), dbName)

	return dbPath
}
