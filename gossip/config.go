package gossip

import (
	"fmt"
	"math/big"
	"path/filepath"
	"time"

	"github.com/Fantom-foundation/lachesis-base/gossip/dagprocessor"
	"github.com/Fantom-foundation/lachesis-base/gossip/itemsfetcher"
	"github.com/Fantom-foundation/lachesis-base/inter/dag"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/utils/cachescale"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/mrmikeo/Xpense/eventcheck/heavycheck"
	"github.com/mrmikeo/Xpense/gossip/evmstore"
	"github.com/mrmikeo/Xpense/gossip/filters"
	"github.com/mrmikeo/Xpense/gossip/gasprice"
	"github.com/mrmikeo/Xpense/gossip/protocols/blockrecords/brprocessor"
	"github.com/mrmikeo/Xpense/gossip/protocols/blockrecords/brstream/brstreamleecher"
	"github.com/mrmikeo/Xpense/gossip/protocols/blockrecords/brstream/brstreamseeder"
	"github.com/mrmikeo/Xpense/gossip/protocols/blockvotes/bvprocessor"
	"github.com/mrmikeo/Xpense/gossip/protocols/blockvotes/bvstream/bvstreamleecher"
	"github.com/mrmikeo/Xpense/gossip/protocols/blockvotes/bvstream/bvstreamseeder"
	"github.com/mrmikeo/Xpense/gossip/protocols/dag/dagstream/dagstreamleecher"
	"github.com/mrmikeo/Xpense/gossip/protocols/dag/dagstream/dagstreamseeder"
	"github.com/mrmikeo/Xpense/gossip/protocols/epochpacks/epprocessor"
	"github.com/mrmikeo/Xpense/gossip/protocols/epochpacks/epstream/epstreamleecher"
	"github.com/mrmikeo/Xpense/gossip/protocols/epochpacks/epstream/epstreamseeder"
)

const nominalSize uint = 1

type (
	// ProtocolConfig is config for p2p protocol
	ProtocolConfig struct {
		// 0/M means "optimize only for throughput", N/0 means "optimize only for latency", N/M is a balanced mode

		LatencyImportance    int
		ThroughputImportance int

		EventsSemaphoreLimit dag.Metric
		BVsSemaphoreLimit    dag.Metric
		MsgsSemaphoreLimit   dag.Metric
		MsgsSemaphoreTimeout time.Duration

		ProgressBroadcastPeriod time.Duration

		DagProcessor dagprocessor.Config
		BvProcessor  bvprocessor.Config
		BrProcessor  brprocessor.Config
		EpProcessor  epprocessor.Config

		DagFetcher       itemsfetcher.Config
		TxFetcher        itemsfetcher.Config
		DagStreamLeecher dagstreamleecher.Config
		DagStreamSeeder  dagstreamseeder.Config
		BvStreamLeecher  bvstreamleecher.Config
		BvStreamSeeder   bvstreamseeder.Config
		BrStreamLeecher  brstreamleecher.Config
		BrStreamSeeder   brstreamseeder.Config
		EpStreamLeecher  epstreamleecher.Config
		EpStreamSeeder   epstreamseeder.Config

		MaxInitialTxHashesSend   int
		MaxRandomTxHashesSend    int
		RandomTxHashesSendPeriod time.Duration

		PeerCache PeerCacheConfig
	}

	// Config for the gossip service.
	Config struct {
		FilterAPI filters.Config

		// This can be set to list of enrtree:// URLs which will be queried
		// for nodes to connect to.
		OperaDiscoveryURLs []string

		TxIndex bool // Whether to enable indexing transactions and receipts or not

		// Protocol options
		Protocol ProtocolConfig

		HeavyCheck heavycheck.Config

		// Gas Price Oracle options
		GPO gasprice.Config

		// RPCGasCap is the global gas cap for eth-call variants.
		RPCGasCap uint64 `toml:",omitempty"`

		// RPCEVMTimeout is the global timeout for eth-call.
		RPCEVMTimeout time.Duration

		// RPCTxFeeCap is the global transaction fee(price * gaslimit) cap for
		// send-transction variants. The unit is ether.
		RPCTxFeeCap float64 `toml:",omitempty"`

		// RPCTimeout is a global time limit for RPC methods execution.
		RPCTimeout time.Duration

		// allows only for EIP155 transactions.
		AllowUnprotectedTxs bool

		// BatchRequestLimit is maximum number of requests in batch.
		BatchRequestLimit int

		// JSTracerLimit is a global JS engine limit for RPC debug methods execution.
		JSTracerLimit int

		// MaxResponseSize is a limit for maximum response size in some RPC calls
		MaxResponseSize int

		RPCBlockExt bool
	}

	StoreCacheConfig struct {
		// Cache size for full events.
		EventsNum  int
		EventsSize uint
		// Cache size for event IDs
		EventsIDsNum int
		// Cache size for full blocks.
		BlocksNum  int
		BlocksSize uint
		// Cache size for history block/epoch states.
		BlockEpochStateNum int

		LlrBlockVotesIndexes int
		LlrEpochVotesIndexes int
	}

	// StoreConfig is a config for store db.
	StoreConfig struct {
		Cache StoreCacheConfig
		// EVM is EVM store config
		EVM               evmstore.StoreConfig
		MaxNonFlushedSize int
		MaxNonFlushedPeriod time.Duration
	}
)

type PeerCacheConfig struct {
	MaxKnownTxs    int // Maximum transactions hashes to keep in the known list (prevent DOS)
	MaxKnownEvents int // Maximum event hashes to keep in the known list (prevent DOS)
	// MaxQueuedItems is the maximum number of items to queue up before
	// dropping broadcasts. This is a sensitive number as a transaction list might
	// contain a single transaction, or thousands.
	MaxQueuedItems idx.Event
	MaxQueuedSize  uint64
}

// DefaultConfig returns the default configurations for the gossip service.
func DefaultConfig(scale cachescale.Func) Config {
	cfg := Config{
		FilterAPI: filters.DefaultConfig(),

		TxIndex: true,

		HeavyCheck: heavycheck.DefaultConfig(),

		Protocol: ProtocolConfig{
			LatencyImportance:    60,
			ThroughputImportance: 40,
			MsgsSemaphoreLimit: dag.Metric{
				Num:  scale.Events(1000),
				Size: scale.U64(30 * opt.MiB),
			},
			EventsSemaphoreLimit: dag.Metric{
				Num:  scale.Events(10000),
				Size: scale.U64(30 * opt.MiB),
			},
			BVsSemaphoreLimit: dag.Metric{
				Num:  scale.Events(5000),
				Size: scale.U64(15 * opt.MiB),
			},
			MsgsSemaphoreTimeout:    10 * time.Second,
			ProgressBroadcastPeriod: 10 * time.Second,

			DagProcessor: dagprocessor.DefaultConfig(scale),
			BvProcessor:  bvprocessor.DefaultConfig(scale),
			BrProcessor:  brprocessor.DefaultConfig(scale),
			EpProcessor:  epprocessor.DefaultConfig(scale),
			DagFetcher: itemsfetcher.Config{
				ForgetTimeout:       1 * time.Minute,
				ArriveTimeout:       1000 * time.Millisecond,
				GatherSlack:         100 * time.Millisecond,
				HashLimit:           20000,
				MaxBatch:            scale.I(512),
				MaxQueuedBatches:    scale.I(32),
				MaxParallelRequests: 192,
			},
			TxFetcher: itemsfetcher.Config{
				ForgetTimeout:       1 * time.Minute,
				ArriveTimeout:       1000 * time.Millisecond,
				GatherSlack:         100 * time.Millisecond,
				HashLimit:           10000,
				MaxBatch:            scale.I(512),
				MaxQueuedBatches:    scale.I(32),
				MaxParallelRequests: 64,
			},
			DagStreamLeecher:         dagstreamleecher.DefaultConfig(),
			DagStreamSeeder:          dagstreamseeder.DefaultConfig(scale),
			BvStreamLeecher:          bvstreamleecher.DefaultConfig(),
			BvStreamSeeder:           bvstreamseeder.DefaultConfig(scale),
			BrStreamLeecher:          brstreamleecher.DefaultConfig(),
			BrStreamSeeder:           brstreamseeder.DefaultConfig(scale),
			EpStreamLeecher:          epstreamleecher.DefaultConfig(),
			EpStreamSeeder:           epstreamseeder.DefaultConfig(scale),
			MaxInitialTxHashesSend:   20000,
			MaxRandomTxHashesSend:    250, // match softLimitItems to fit into one message
			RandomTxHashesSendPeriod: 1 * time.Second,
			PeerCache:                DefaultPeerCacheConfig(scale),
		},

		RPCEVMTimeout: 5 * time.Second,

		GPO: gasprice.Config{
			MaxGasPrice:      gasprice.DefaultMaxGasPrice,
			MinGasPrice:      new(big.Int),
			DefaultCertainty: 0.5 * gasprice.DecimalUnit,
		},

		RPCBlockExt: true,

		RPCGasCap:   50000000,
		RPCTxFeeCap: 100, // 100 FTM
		RPCTimeout:  5 * time.Second,

		BatchRequestLimit: 1000,

		JSTracerLimit: 1000,

		MaxResponseSize: 25 * 1024 * 1024,
	}
	sessionCfg := cfg.Protocol.DagStreamLeecher.Session
	cfg.Protocol.DagProcessor.EventsBufferLimit.Num = idx.Event(sessionCfg.ParallelChunksDownload)*
		idx.Event(sessionCfg.DefaultChunkItemsNum) + softLimitItems
	cfg.Protocol.DagProcessor.EventsBufferLimit.Size = uint64(sessionCfg.ParallelChunksDownload)*sessionCfg.DefaultChunkItemsSize + 8*opt.MiB
	cfg.Protocol.DagStreamLeecher.MaxSessionRestart = 4 * time.Minute
	cfg.Protocol.DagFetcher.ArriveTimeout = 4 * time.Second
	cfg.Protocol.DagFetcher.HashLimit = 10000
	cfg.Protocol.TxFetcher.HashLimit = 10000

	return cfg
}

func (c *Config) Validate() error {
	p := c.Protocol
	defaultChunkSize := dag.Metric{idx.Event(p.DagStreamLeecher.Session.DefaultChunkItemsNum), p.DagStreamLeecher.Session.DefaultChunkItemsSize}
	if defaultChunkSize.Num > hardLimitItems-1 {
		return fmt.Errorf("DefaultChunkSize.Num has to be at not greater than %d", hardLimitItems-1)
	}
	if defaultChunkSize.Size > protocolMaxMsgSize/2 {
		return fmt.Errorf("DefaultChunkSize.Num has to be at not greater than %d", protocolMaxMsgSize/2)
	}
	if p.EventsSemaphoreLimit.Num < 2*defaultChunkSize.Num ||
		p.EventsSemaphoreLimit.Size < 2*defaultChunkSize.Size {
		return fmt.Errorf("EventsSemaphoreLimit has to be at least 2 times greater than %s (DefaultChunkSize)", defaultChunkSize.String())
	}
	if p.EventsSemaphoreLimit.Num < 2*p.DagProcessor.EventsBufferLimit.Num ||
		p.EventsSemaphoreLimit.Size < 2*p.DagProcessor.EventsBufferLimit.Size {
		return fmt.Errorf("EventsSemaphoreLimit has to be at least 2 times greater than %s (EventsBufferLimit)", p.DagProcessor.EventsBufferLimit.String())
	}
	if p.EventsSemaphoreLimit.Size < 2*protocolMaxMsgSize {
		return fmt.Errorf("EventsSemaphoreLimit.Size has to be at least %d", 2*protocolMaxMsgSize)
	}
	if p.MsgsSemaphoreLimit.Size < protocolMaxMsgSize {
		return fmt.Errorf("MsgsSemaphoreLimit.Size has to be at least %d", protocolMaxMsgSize)
	}
	if p.DagProcessor.EventsBufferLimit.Size < protocolMaxMsgSize {
		return fmt.Errorf("EventsBufferLimit.Size has to be at least %d", protocolMaxMsgSize)
	}

	return nil
}

// DefaultStoreConfig for product.
func DefaultStoreConfig(scale cachescale.Func) StoreConfig {
	return StoreConfig{
		Cache: StoreCacheConfig{
			EventsNum:            scale.I(5000),
			EventsSize:           scale.U(6 * opt.MiB),
			EventsIDsNum:         scale.I(100000),
			BlocksNum:            scale.I(5000),
			BlocksSize:           scale.U(512 * opt.KiB),
			BlockEpochStateNum:   scale.I(8),
			LlrBlockVotesIndexes: scale.I(100),
			LlrEpochVotesIndexes: scale.I(5),
		},
		EVM:                 evmstore.DefaultStoreConfig(scale),
		MaxNonFlushedSize:   21*opt.MiB + scale.I(2*opt.MiB),
		MaxNonFlushedPeriod: 30 * time.Minute,
	}
}

// MemTestStoreConfig is for tests or inmemory.
func MemTestStoreConfig(tmpDir string) StoreConfig {
	cfg := DefaultStoreConfig(cachescale.Ratio{Base: 10, Target: 1})
	cfg.EVM.StateDb.Directory = filepath.Join(tmpDir, "carmen")
	cfg.EVM.StateDb.LiveCache = 100 // bytes, to be overridden by the minimal value
	cfg.EVM.StateDb.ArchiveCache = 100 // bytes, to be overridden by the minimal value
	return cfg
}

func DefaultPeerCacheConfig(scale cachescale.Func) PeerCacheConfig {
	return PeerCacheConfig{
		MaxKnownTxs:    24576*3/4 + scale.I(24576/4),
		MaxKnownEvents: 24576*3/4 + scale.I(24576/4),
		MaxQueuedItems: 4096*3/4 + scale.Events(4096/4),
		MaxQueuedSize:  protocolMaxMsgSize*3/4 + 1024 + scale.U64(protocolMaxMsgSize/4),
	}
}
