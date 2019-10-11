package gossip

import (
	lru "github.com/hashicorp/golang-lru"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/Fantom-foundation/go-lachesis/hash"
	"github.com/Fantom-foundation/go-lachesis/kvdb"
	"github.com/Fantom-foundation/go-lachesis/kvdb/flushable"
	"github.com/Fantom-foundation/go-lachesis/kvdb/memorydb"
	"github.com/Fantom-foundation/go-lachesis/kvdb/no_key_is_err"
	"github.com/Fantom-foundation/go-lachesis/kvdb/table"
	"github.com/Fantom-foundation/go-lachesis/logger"
)

// StoreConfig is a config for Store.
type StoreConfig struct {
	// LRU cache size for Events (stored by pointer).
	EventsCacheSize			int

	// LRU cache size for Epoch (EventHeaderData) (stored by pointer).
	EventsHeadersCacheSize 	int

	// LRU cache size for Block (stored by pointer).
	BlockCacheSize			int

	// LRU cache size for PackInfos (stored by value).
	PackInfosCacheSize		int

	// LRU cache size for Receipts  (stored by value).
	ReceiptsCacheSize		int

	// LRU cache size for TxPositions (stored by pointer).
	TxPositionsCacheSize	int
}

// Store is a node persistent storage working over physical key-value database.
type Store struct {
	dbs *flushable.SyncedPool

	mainDb kvdb.KeyValueStore
	table  struct {
		Peers     kvdb.KeyValueStore `table:"peer_"`
		Events    kvdb.KeyValueStore `table:"event_"`
		Blocks    kvdb.KeyValueStore `table:"block_"`
		PackInfos kvdb.KeyValueStore `table:"packinfo_"`
		Packs     kvdb.KeyValueStore `table:"pack_"`
		PacksNum  kvdb.KeyValueStore `table:"packs_num_"`

		// API-only tables
		BlockHashes kvdb.KeyValueStore `table:"block_h_"`
		Receipts    kvdb.KeyValueStore `table:"receipts_"`
		TxPositions kvdb.KeyValueStore `table:"tx_p_"`

		TmpDbs kvdb.KeyValueStore `table:"tmpdbs_"`

		Evm      ethdb.Database
		EvmState state.Database
	}

	cache struct {
		Events			*lru.Cache
		EventsHeaders 	*lru.Cache
		Blocks			*lru.Cache
		PackInfos		*lru.Cache
		Receipts		*lru.Cache
		TxPositions		*lru.Cache
	}

	tmpDbs

	logger.Instance
}

// NewStore creates store over key-value db.
func NewStore(dbs *flushable.SyncedPool, cfg *StoreConfig) *Store {
	s := &Store{
		dbs:      dbs,
		mainDb:   dbs.GetDb("gossip-main"),
		Instance: logger.MakeInstance(),
	}

	table.MigrateTables(&s.table, s.mainDb)

	evmTable := no_key_is_err.Wrap(table.New(s.mainDb, []byte("evm_"))) // ETH expects that "not found" is an error
	s.table.Evm = rawdb.NewDatabase(evmTable)
	s.table.EvmState = state.NewDatabase(s.table.Evm)

	s.initTmpDbs()
	s.initLRUCache(cfg)

	return s
}

// NewMemStore creates store over memory map.
func NewMemStore() *Store {
	mems := memorydb.NewProdicer("")
	dbs := flushable.NewSyncedPool(mems)

	defaultTestStoreCfg := StoreConfig{
		EventsCacheSize:        100,
		EventsHeadersCacheSize: 1000,
		BlockCacheSize:			100,
		PackInfosCacheSize:		100,
		ReceiptsCacheSize:		100,
		TxPositionsCacheSize:	100,
	}

	return NewStore(dbs, &defaultTestStoreCfg)
}

// Close leaves underlying database.
func (s *Store) Close() {
	table.MigrateTables(&s.table, nil)
	s.mainDb.Close()
}

// Commit changes.
func (s *Store) Commit(e hash.Event) {
	s.dbs.FlushIfNeeded(e.Bytes())
}

// StateDB returns state database.
func (s *Store) StateDB(from common.Hash) *state.StateDB {
	db, err := state.New(common.Hash(from), s.table.EvmState)
	if err != nil {
		s.Log.Crit("Failed to open state", "err", err)
	}
	return db
}

/*
 * Utils:
 */

// set RLP value
func (s *Store) set(table kvdb.KeyValueStore, key []byte, val interface{}) {
	buf, err := rlp.EncodeToBytes(val)
	if err != nil {
		s.Log.Crit("Failed to encode rlp", "err", err)
	}

	if err := table.Put(key, buf); err != nil {
		s.Log.Crit("Failed to put key-value", "err", err)
	}
}

// get RLP value
func (s *Store) get(table kvdb.KeyValueStore, key []byte, to interface{}) interface{} {
	buf, err := table.Get(key)
	if err != nil {
		s.Log.Crit("Failed to get key-value", "err", err)
	}
	if buf == nil {
		return nil
	}

	err = rlp.DecodeBytes(buf, to)
	if err != nil {
		s.Log.Crit("Failed to decode rlp", "err", err)
	}
	return to
}

func (s *Store) has(table kvdb.KeyValueStore, key []byte) bool {
	res, err := table.Has(key)
	if err != nil {
		s.Log.Crit("Failed to get key", "err", err)
	}
	return res
}

// Init LRU cache.
func (s *Store) initLRUCache(cfg *StoreConfig) {
	if cfg == nil {
		return
	}

	s.cache.Events 			= s.initOneLRUCache(cfg.EventsCacheSize)
	s.cache.EventsHeaders 	= s.initOneLRUCache(cfg.EventsHeadersCacheSize)
	s.cache.Blocks 			= s.initOneLRUCache(cfg.BlockCacheSize)
	s.cache.PackInfos 		= s.initOneLRUCache(cfg.PackInfosCacheSize)
	s.cache.Receipts 		= s.initOneLRUCache(cfg.ReceiptsCacheSize)
	s.cache.TxPositions 	= s.initOneLRUCache(cfg.TxPositionsCacheSize)

	return
}

func (s *Store) initOneLRUCache(size int) *lru.Cache {
	if size > 0 {
		cache, err := lru.New(size)
		if err != nil {
			s.Log.Error("Error create LRU cache", "err", err)
			return nil
		}
		return cache
	}
	return nil
}
