package chain

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/pkg/errors"
)

/*
QueryCurrencyReward retrieves the current currency reward allocation from treasury storage.
Queries the "CurrencyReward" entry under "CessTreasury" module in chain storage.

Parameters:
  - block: Target block number for historical query

Returns:
  - types.U128: 128-bit unsigned integer reward value
  - error: Wrapped storage access error
*/
func (c *Client) QueryCurrencyReward(block uint32) (types.U128, error) {

	data, err := QueryStorage[types.U128](c, block, "CessTreasury", "CurrencyReward")
	if err != nil {
		return data, errors.Wrap(err, "query currency reward error")
	}
	return data, nil
}

/*
QueryEraReward fetches the accumulated reward for the current era.
Accesses "EraReward" storage entry in "CessTreasury" module.

Parameters:
  - block: Block number for state query

Returns:
  - types.U128: Era-specific reward amount
  - error: Wrapped chain interaction error
*/
func (c *Client) QueryEraReward(block uint32) (types.U128, error) {
	data, err := QueryStorage[types.U128](c, block, "CessTreasury", "EraReward")
	if err != nil {
		return data, errors.Wrap(err, "query era reward error")
	}
	return data, nil
}

/*
QueryReserveReward gets the treasury's reserved reward pool balance.
Reads "ReserveReward" entry from runtime storage.

Parameters:
  - block: Target block for state inspection

Returns:
  - types.U128: Reserve pool balance
  - error: Enhanced error with operation context
*/
func (c *Client) QueryReserveReward(block uint32) (types.U128, error) {
	data, err := QueryStorage[types.U128](c, block, "CessTreasury", "ReserveReward")
	if err != nil {
		return data, errors.Wrap(err, "query reserve reward error")
	}
	return data, nil
}

/*
QueryRoundReward retrieves reward distribution details for a specific era.
Requires SCALE-encoded era parameter to access "RoundReward" storage entry.

Parameters:
  - era: Target reward distribution era
  - block: Block number for state query

Returns:
  - RoundRewardType: Structured reward distribution data
  - error: Composite error including parameter encoding failures
*/
func (c *Client) QueryRoundReward(era, block uint32) (RoundRewardType, error) {
	param, err := codec.Encode(era)
	if err != nil {
		return RoundRewardType{}, errors.Wrap(err, "query round reward error")
	}
	data, err := QueryStorage[RoundRewardType](c, block, "CessTreasury", "RoundReward", param)
	if err != nil {
		return data, errors.Wrap(err, "query round reward error")
	}
	return data, nil
}
