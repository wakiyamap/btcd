package database

import (
	"path/filepath"

	"github.com/wakiyamap/monautil"
	"github.com/wakiyamap/monad/wire"
	"github.com/wakiyamap/monad/chaincfg"
	"github.com/btcsuite/goleveldb/leveldb"
)

const (
	// userCheckpointDbNamePrefix is the prefix for the monad block database.
	userCheckpointDbNamePrefix = "usercheckpoints"
)

var (
	monadHomeDir    = monautil.AppDataDir("monad", false)
	defaultDataDir  = filepath.Join(monadHomeDir, "data")
	activeNetParams = &chaincfg.MainNetParams
)

func CheckpointDbOpen(dbpath string) (*leveldb.DB , error) {
	var err error
	db, err := leveldb.OpenFile(dbpath, nil)
	if err != nil {
		return nil, err
	}
	return db, nil
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
	dbName := userCheckpointDbNamePrefix + "_" + "leveldb"
	dbPath = filepath.Join(defaultDataDir, netName(activeNetParams), dbName)

	return dbPath
}

