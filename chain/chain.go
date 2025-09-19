package chain

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	rpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/retriever"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry/state"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/centrifuge/go-substrate-rpc-client/v4/xxhash"
	"github.com/pkg/errors"
)

type Client struct {
	Rpcs     []string
	ConnNum  int
	nonceMap *sync.Map
	KeyringManager
	GenesisBlockHash types.Hash
	RuntimeVersion   *types.RuntimeVersion
	// *rpc.SubstrateAPI
	connMg        *ConnManager
	Retriever     retriever.EventRetriever
	Timeout       time.Duration
	Metadata      *types.Metadata
	requestQueue  chan struct{}
	CallRegistery registry.CallRegistry
}

type Option func(*Client) error

type ConnManager struct {
	rpcs    []string
	connNum int
	index   *atomic.Uint32
	conns   []*rpc.SubstrateAPI
}

func NewConnManager(rpcs []string, connNum int) (*ConnManager, error) {
	if connNum <= 0 || connNum > 128 {
		connNum = 4
	}
	cm := &ConnManager{
		rpcs:    rpcs,
		connNum: connNum,
		index:   &atomic.Uint32{},
	}
	err := cm.initConns()
	return cm, errors.Wrap(err, "new connection manager error")
}

func (cm *ConnManager) initConns() error {
	lens := len(cm.rpcs)
	if lens < 1 || cm.connNum < 1 {
		return errors.New("params not initialized")
	}
	cm.conns = make([]*rpc.SubstrateAPI, cm.connNum)
	for i := range cm.connNum {
		cli, err := rpc.NewSubstrateAPI(cm.rpcs[i%lens])
		if err != nil {
			for j := range lens {
				cli, err = rpc.NewSubstrateAPI(cm.rpcs[j])
				if err == nil {
					break
				}
			}
			if err != nil {
				return err
			}
		}
		cm.conns[i] = cli
	}
	return nil
}

func (cm *ConnManager) GetConn() (*rpc.SubstrateAPI, error) {
	if len(cm.conns) <= 0 {
		return nil, errors.New("no connection available")
	}
	return cm.conns[(cm.index.Add(1)-1)%uint32(cm.connNum)], nil
}

func (cm *ConnManager) RunConnRefreshServer(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		for i, conn := range cm.conns {
			if _, err := conn.RPC.Chain.GetBlockLatest(); err == nil {
				continue
			}
			cli, err := rpc.NewSubstrateAPI(cm.rpcs[i%len(cm.rpcs)])
			if err == nil {
				cm.conns[i] = cli
				continue
			}
			log.Println(fmt.Errorf("[chain client refresh] reconnecting to RPC %s failed: %v", cm.rpcs[i%len(cm.rpcs)], err))
			for j := range len(cm.rpcs) {
				cli, err = rpc.NewSubstrateAPI(cm.rpcs[j])
				if err == nil {
					cm.conns[i] = cli
					break
				}
				log.Println(fmt.Errorf("[chain client refresh] reconnecting to RPC %s failed: %v", cm.rpcs[j], err))
			}
		}
	}
}

// OptionWithRpcs configures the client with a list of RPC endpoints.
// Parameters:
//
//	rpcs - List of RPC server addresses (e.g., "ws://localhost:9944")
//
// Returns:
//
//	Functional option to apply to the client
func OptionWithRpcs(rpcs []string) Option {
	return func(c *Client) error {
		c.Rpcs = rpcs
		return nil
	}
}

func OptionWithConnNum(num int) Option {
	return func(c *Client) error {
		if num <= 0 || num > 128 {
			num = 4
		}
		c.ConnNum = num
		return nil
	}
}

// OptionWithAccounts configures the client with keyring accounts from mnemonics.
// Parameters:
//
//	mnemonics - List of BIP-39 mnemonic phrases for account recovery
//
// Returns:
//
//	Functional option to apply to the client
func OptionWithAccounts(mnemonics []string) Option {
	return func(c *Client) error {
		keys := make([]signature.KeyringPair, 0, len(mnemonics))
		for _, m := range mnemonics {
			key, err := signature.KeyringPairFromSecret(m, 0)
			if err != nil {
				return err
			}
			keys = append(keys, key)
		}
		c.KeyringManager = NewKeyrings(keys...)
		return nil
	}
}

// OptionWithTimeout sets the default timeout for chain operations.
// Parameters:
//
//	timeout - Timeout duration (minimum 15 seconds if <=0)
//
// Returns:
//
//	Functional option to apply to the client
func OptionWithTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		if timeout <= 0 {
			timeout = time.Second * 15
		}
		c.Timeout = timeout
		return nil
	}
}

// NewLightCessClient creates a lightweight CESS client with a single account.
// Parameters:
//
//	mnemonic - BIP-39 mnemonic for the primary account
//	rpcs - List of RPC endpoints to connect to
//
// Returns:
//
//	Lightweight Client instance
//	Error if client initialization fails
func NewLightCessClient(mnemonic string, rpcs []string) (*Client, error) {
	cli, err := NewClient(
		OptionWithRpcs(rpcs),
		OptionWithAccounts([]string{mnemonic}),
	)
	if err != nil {
		return cli, errors.Wrap(err, "new light cess client error")
	}
	return cli, nil
}

// NewClient creates a full-featured CESS chain client with configurable options.
// Initializes RPC connection, retrieves metadata, genesis hash, and runtime version.
// Parameters:
//
//	opts - List of functional options for client configuration
//
// Returns:
//
//	Configured Client instance
//	Error if any initialization step fails
func NewClient(opts ...Option) (*Client, error) {
	client := &Client{nonceMap: &sync.Map{}, requestQueue: make(chan struct{}, 150)}
	for _, opt := range opts {
		if err := opt(client); err != nil {
			return client, errors.Wrap(err, "new cess chain client error")
		}
	}
	// err := client.RefreshSubstrateApi(true)
	// if err != nil {
	// 	return client, errors.Wrap(err, "new cess chain client error")
	// }
	mg, err := NewConnManager(client.Rpcs, client.ConnNum)
	if err != nil {
		return client, errors.Wrap(err, "new cess chain client error")
	}
	client.connMg = mg
	conn, err := mg.GetConn()
	if err != nil {
		return client, errors.Wrap(err, "new cess chain client error")
	}
	client.Metadata, err = conn.RPC.State.GetMetadataLatest()
	if err != nil {
		return client, errors.Wrap(err, "new cess chain client error")
	}
	client.GenesisBlockHash, err = conn.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return client, errors.Wrap(err, "new cess chain client error")
	}
	client.RuntimeVersion, err = conn.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return client, errors.Wrap(err, "new cess chain client error")
	}
	client.Retriever, err = retriever.NewDefaultEventRetriever(
		state.NewEventProvider(conn.RPC.State),
		conn.RPC.State,
	)
	if err != nil {
		return client, errors.Wrap(err, "new cess chain client error")
	}
	client.CallRegistery, err = registry.NewFactory().CreateCallRegistry(client.Metadata)
	if err != nil {
		return client, errors.Wrap(err, "new cess chain client error")
	}
	if client.Timeout <= 0 {
		client.Timeout = time.Second * 30
	}
	go mg.RunConnRefreshServer(context.Background())
	return client, nil
}

func (c *Client)GetConnectionClient()(*rpc.SubstrateAPI,error){
	return c.connMg.GetConn()
}

// ParseSystemEventError converts a module error into a human-readable error message.
// Uses client metadata to resolve error index and code to error name/description.
// Parameters:
//
//	t - ModuleError struct containing error index and code
//
// Returns:
//
//	Formatted error message
//	Error if metadata lookup fails
func (c *Client) ParseSystemEventError(t types.ModuleError) error {
	e, err := c.Metadata.FindError(t.Index, t.Error)
	if err != nil {
		return errors.Wrap(err, "extrinsic failed")
	}
	if e == nil || e.Name == "" || e.Name == "InvalidSpecName" {
		return errors.Wrap(errors.New("unknown event type"), "extrinsic failed")
	}
	return errors.Wrap(fmt.Errorf("%s: %s", e.Name, e.Value), "extrinsic failed")
}

// SubmitExtrinsic signs and submits an extrinsic to the blockchain network.
// Monitors transaction status, decodes specified event, and handles timeout.
// Parameters:
//
//	keypair - Account keyring pair for signing the extrinsic
//	call - Substrate call data to include in the extrinsic
//	eventName - Name of event to decode (empty for no event decoding)
//	event - Pointer to struct for decoded event data (optional)
//	timeout - Maximum time to wait for transaction confirmation
//
// Returns:
//
//	Block hash containing the transaction
//	Error if signing, submission, or event decoding fails
func (c *Client) SubmitExtrinsic(caller *signature.KeyringPair, call types.Call, eventName string, event any, timeout time.Duration) (string, error) {

	var (
		hash string
		err  error
	)
	c.requestQueue <- struct{}{}
	defer func() { <-c.requestQueue }()
	keypair, err := c.GetCaller(caller)
	if err != nil {
		return hash, errors.Wrap(err, "submit extrinsic error")
	}
	conn, err := c.connMg.GetConn()
	if err != nil {
		return hash, errors.Wrap(err, "submit extrinsic error")
	}

	ext := types.NewExtrinsic(call)
	nonce, err := c.GetCallerNonce(&keypair, conn)
	if err != nil {
		c.PutCaller(&keypair)
		return hash, errors.Wrap(err, "submit extrinsic error")
	}

	o := types.SignatureOptions{
		BlockHash:          c.GenesisBlockHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        c.GenesisBlockHash,
		Nonce:              types.NewUCompactFromUInt(nonce),
		SpecVersion:        c.RuntimeVersion.SpecVersion,
		Tip:                types.NewUCompactFromUInt(0),
		TransactionVersion: c.RuntimeVersion.TransactionVersion,
	}

	err = ext.Sign(keypair, o)
	if err != nil {
		c.PutCaller(&keypair)
		return hash, errors.Wrap(err, "submit extrinsic error")
	}

	c.PutCaller(&keypair)
	sub, err := conn.RPC.Author.SubmitAndWatchExtrinsic(ext)
	if err != nil {
		if strings.Contains(err.Error(), "Priority is too low") {
			c.UpdateCallerNonce(&keypair, conn)
		}
		return hash, errors.Wrap(err, "submit extrinsic error")
	}
	defer sub.Unsubscribe()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case status := <-sub.Chan():
			if !status.IsInBlock {
				continue
			}
			hash = status.AsInBlock.Hex()
			if eventName == "" {
				return hash, nil
			}

			events, err := c.Retriever.GetEvents(status.AsInBlock)
			if err != nil {
				return hash, errors.Wrap(err, "submit extrinsic error")
			}
			e, err := c.ParseTxResult(keypair, events, eventName)
			if err != nil {
				return hash, errors.Wrap(err, "submit extrinsic error")
			}
			if e != nil && event != nil {
				if err = DecodeEvent(e, event); err != nil {
					return hash, errors.Wrap(err, "submit extrinsic error")
				}
			}
			return hash, nil
		case err = <-sub.Err():
			return hash, errors.Wrap(err, "submit extrinsic error")
		case <-timer.C:
			return hash, errors.Wrap(errors.New("timeout"), "submit extrinsic error")
		}
	}
}

// QueryStorage retrieves a single storage entry from the blockchain.
// Parameters:
//
//	T - Generic type for the storage data
//	c - Client instance for chain interaction
//	block - Block number to query (0 for latest)
//	prefix - Storage module prefix (e.g., "System")
//	method - Storage function name (e.g., "Account")
//	args - Optional arguments for storage key generation
//
// Returns:
//
//	Decoded storage data of type T
//	Error if key creation, storage retrieval, or decoding fails
func QueryStorage[T any](c *Client, block uint32, prefix, method string, args ...[]byte) (T, error) {
	var (
		ok   bool
		err  error
		key  types.StorageKey
		data T
	)
	key, err = types.CreateStorageKey(c.Metadata, prefix, method, args...)
	if err != nil {
		return data, errors.Wrap(err, "query storage error")
	}
	conn, err := c.connMg.GetConn()
	if err != nil {
		return data, errors.Wrap(err, "query storage error")
	}
	if block == 0 {
		ok, err = conn.RPC.State.GetStorageLatest(key, &data)
	} else {
		var hash types.Hash
		hash, err = conn.RPC.Chain.GetBlockHash(uint64(block))
		if err != nil {
			return data, errors.Wrap(err, "query storage error")
		}
		ok, err = conn.RPC.State.GetStorage(key, &data, hash)
	}
	if err != nil {
		return data, errors.Wrap(err, "query storage error")
	}
	if !ok {
		return data, errors.Wrap(errors.New("data not found"), "query storage error")
	}
	return data, nil
}

// QueryStorages retrieves multiple storage entries matching a prefix.
// Parameters:
//
//	T - Generic type for the storage data
//	c - Client instance for chain interaction
//	block - Block number to query (0 for latest)
//	prefix - Storage module prefix
//	method - Storage function name
//
// Returns:
//
//	Slice of decoded storage data entries
//	Error if key retrieval, storage query, or decoding fails
func QueryStorages[T any](c *Client, block uint32, prefix, method string) ([]T, error) {
	var (
		err   error
		keys  []types.StorageKey
		set   []types.StorageChangeSet
		datas []T
	)
	conn, err := c.connMg.GetConn()
	if err != nil {
		return datas, errors.Wrap(err, "query storages error")
	}
	keys, err = conn.RPC.State.GetKeysLatest(createPrefixedKey(method, prefix))
	if err != nil {
		return datas, errors.Wrap(err, "query storages error")
	}
	if block == 0 {
		set, err = conn.RPC.State.QueryStorageAtLatest(keys)
	} else {
		var hash types.Hash
		hash, err = conn.RPC.Chain.GetBlockHash(uint64(block))
		if err != nil {
			return datas, errors.Wrap(err, "query storages error")
		}
		set, err = conn.RPC.State.QueryStorageAt(keys, hash)
	}

	if err != nil {
		return datas, errors.Wrap(err, "query storages error")
	}
	for _, elem := range set {
		for _, change := range elem.Changes {
			var data T
			if err := codec.Decode(change.StorageData, &data); err != nil {
				continue
			}
			datas = append(datas, data)
		}
	}
	return datas, nil
}

// GetCaller retrieves the signing keyring pair for transactions.
// Uses provided caller if not nil; otherwise selects from client's keyring.
// Parameters:
//
//	caller - Optional specific keyring pair to use
//
// Returns:
//
//	Keyring pair for transaction signing
//	Error if no valid caller or keyring configured
func (c *Client) GetCaller(caller *signature.KeyringPair) (signature.KeyringPair, error) {
	var key signature.KeyringPair
	if caller == nil {
		if c.KeyringManager == nil {
			return key, errors.New("invalid tx sender")
		}
		key = c.GetKeyInOrder()
	} else {
		key = *caller
	}
	return key, nil
}

func (c *Client) PutCaller(caller *signature.KeyringPair) {
	if caller != nil {
		c.PutKey(caller.Address)
	}
}

func (c *Client) GetCallerNonce(caller *signature.KeyringPair, conn *rpc.SubstrateAPI) (uint64, error) {
	if caller == nil {
		return 0, errors.New("invalid caller")
	}
	if v, ok := c.nonceMap.Load(caller.Address); ok {
		noncer, ok := v.(*atomic.Uint64)
		if !ok {
			return 0, errors.New("invalid nonce value")
		}
		return noncer.Add(1) - 1, nil
	}
	if err := c.UpdateCallerNonce(caller, conn); err != nil {
		return 0, err
	}
	return c.GetCallerNonce(caller, conn)
}

func (c *Client) UpdateCallerNonce(caller *signature.KeyringPair, conn *rpc.SubstrateAPI) error {
	if caller == nil {
		return errors.New("invalid caller")
	}

	var accountInfo types.AccountInfo
	key, err := types.CreateStorageKey(c.Metadata, "System", "Account", caller.PublicKey)
	if err != nil {
		return err
	}
	if _, err := conn.RPC.State.GetStorageLatest(key, &accountInfo); err != nil {
		return err
	}
	act, loaded := c.nonceMap.LoadOrStore(caller.Address, &atomic.Uint64{})
	v, ok := act.(*atomic.Uint64)
	if !ok {
		return errors.New("invalid nonce value")
	}
	if !loaded || v.Load() < uint64(accountInfo.Nonce) {
		v.Store(uint64(accountInfo.Nonce))
	}
	return nil
}

func createPrefixedKey(method, prefix string) []byte {
	return append(xxhash.New128([]byte(prefix)).Sum(nil), xxhash.New128([]byte(method)).Sum(nil)...)
}
