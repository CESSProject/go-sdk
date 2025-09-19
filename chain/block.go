package chain

import (
	"reflect"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"
)

type ParsedBlock struct {
	Hash         types.Hash
	Header       types.Header
	Extrinsics   []ParsedExtrinsic
	SystemEvents []ParsedEvent
}

type ParsedExtrinsic struct {
	Name           string
	Hash           string
	NoExtrinsic    bool
	ExtrinsicError error
	Account        types.AccountID
	Raw            types.Extrinsic
	Events         []ParsedEvent
}

type ParsedEvent struct {
	Name  string
	Event any
}

// ParseBlockDataWithBlockNumber retrieves and parses block data by block number
//
// Parameters:
//   - block: Target block number to parse
//
// Returns:
//   - ParsedBlock: Structured block data with extrinsics and events
//   - error: Wrapped error containing original error context
func (c *Client) ParseBlockDataWithBlockNumber(block uint32) (ParsedBlock, error) {
	conn, err := c.connMg.GetConn()
	if err != nil {
		return ParsedBlock{}, errors.Wrap(err, "parse block data with block number error")
	}
	hash, err := conn.RPC.Chain.GetBlockHash(uint64(block))
	if err != nil {
		return ParsedBlock{}, errors.Wrap(err, "parse block data with block number error")
	}
	data, err := c.ParseBlockData(hash)
	if err != nil {
		return data, errors.Wrap(err, "parse block data with block number error")
	}
	return data, nil
}

// ParseBlockData processes raw block data into structured format
//
// Workflow:
//  1. Creates call registry from metadata
//  2. Retrieves raw block data from chain
//  3. Processes extrinsics (transactions):
//     - Generates call hash
//     - Extracts signature information
//  4. Retrieves and processes system events:
//     - Matches events to extrinsics
//     - Handles success/failure statuses
//     - Captures extrinsic errors
//
// Parameters:
//   - hash: Block hash identifier
//
// Returns:
//   - ParsedBlock: Fully parsed block structure
//   - error: Wrapped error with operation context
func (c *Client) ParseBlockData(hash types.Hash) (ParsedBlock, error) {
	parsedBlock := ParsedBlock{
		Hash: hash,
	}
	conn, err := c.connMg.GetConn()
	if err != nil {
		return parsedBlock, errors.Wrap(err, "parse block data error")
	}
	block, err := conn.RPC.Chain.GetBlock(hash)
	if err != nil {
		return parsedBlock, errors.Wrap(err, "parse block data error")
	}

	parsedBlock.Header = block.Block.Header
	for _, e := range block.Block.Extrinsics {
		call := c.CallRegistery[e.Method.CallIndex]
		data, _ := codec.Encode(e.Method)
		h := blake2b.Sum256(data)
		parsedBlock.Extrinsics = append(parsedBlock.Extrinsics, ParsedExtrinsic{
			Hash:        hexutil.Encode(h[:]),
			Name:        call.Name,
			NoExtrinsic: !e.Signature.Signer.IsID,
			Account:     e.Signature.Signer.AsAddress32,
			Raw:         e,
		})
	}

	events, err := c.Retriever.GetEvents(hash)
	if err != nil {
		return parsedBlock, errors.Wrap(err, "parse block data error")
	}

	var eventBuf []ParsedEvent

	index := 0
	for _, e := range events {
		if t, ok := commonEventsTypeMap[e.Name]; ok {
			value := reflect.New(t).Interface()
			if err := DecodeEvent(e, value); err == nil {
				eventBuf = append(eventBuf, ParsedEvent{Name: e.Name, Event: value})
			}
		}
		if len(eventBuf) > 0 && index < len(parsedBlock.Extrinsics) &&
			(e.Name == "System.ExtrinsicSuccess" || e.Name == "System.ExtrinsicFailed") {
			if e.Name == "System.ExtrinsicFailed" && len(eventBuf) > 0 {
				failed, ok := eventBuf[len(eventBuf)-1].Event.(types.EventSystemExtrinsicFailed)
				if ok {
					parsedBlock.Extrinsics[index].ExtrinsicError = c.ParseSystemEventError(failed.DispatchError.ModuleError)
				}
			}
			if parsedBlock.Extrinsics[index].NoExtrinsic {
				parsedBlock.SystemEvents = append(parsedBlock.SystemEvents, eventBuf[:len(eventBuf)-1]...)
				parsedBlock.Extrinsics[index].Events = eventBuf[len(eventBuf)-1:]
			} else {
				parsedBlock.Extrinsics[index].Events = eventBuf
			}
			eventBuf = nil
			index++
		}
	}
	return parsedBlock, nil
}
