package chain

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/pkg/errors"
)

/*
QueryCounterForValidators gets the current validator count from chain storage.
Accesses "CounterForValidators" entry under "Staking" module.

Parameters:
  - block: Target block number for state query

Returns:
  - uint32: Active validator count
  - error: Wrapped storage access error
*/
func (c *Client) QueryCounterForValidators(block uint32) (uint32, error) {
	data, err := QueryStorage[types.U32](c, block, "Staking", "CounterForValidators")
	if err != nil {
		return uint32(data), errors.Wrap(err, "query counter for validators error")
	}
	return uint32(data), nil
}

/*
QueryErasTotalStaking retrieves total staked amount for a specific era.
Requires SCALE-encoded era parameter to query "ErasTotalStake" storage.

Parameters:
  - era: Target staking era
  - block: Block number for historical data

Returns:
  - types.U128: 128-bit total stake value
  - error: Composite error including encoding failures
*/
func (c *Client) QueryErasTotalStaking(era, block uint32) (types.U128, error) {
	param, err := codec.Encode(era)
	if err != nil {
		return types.U128{}, errors.Wrap(err, "query eras total staking error")
	}
	data, err := QueryStorage[types.U128](c, block, "Staking", "ErasTotalStake", param)
	if err != nil {
		return data, errors.Wrap(err, "query eras total staking error")
	}
	return data, nil
}

/*
QueryCurrentEra fetches the latest active era index.
Reads "CurrentEra" entry from runtime storage.

Parameters:
  - block: Block for state inspection

Returns:
  - uint32: Current era number
  - error: Enhanced error with operation context
*/
func (c *Client) QueryCurrentEra(block uint32) (uint32, error) {
	data, err := QueryStorage[types.U32](c, block, "Staking", "CurrentEra")
	if err != nil {
		return uint32(data), errors.Wrap(err, "query current era error")
	}
	return uint32(data), nil
}

/*
QueryErasRewardPoints gets reward distribution details per era.
Queries "ErasRewardPoints" storage with SCALE-encoded era parameter.

Parameters:
  - era: Target reward distribution era
  - block: Query block number

Returns:
  - StakingEraRewardPoints: Structured reward points data
  - error: Wrapped chain interaction error
*/
func (c *Client) QueryErasRewardPoints(era, block uint32) (StakingEraRewardPoints, error) {
	param, err := codec.Encode(era)
	if err != nil {
		return StakingEraRewardPoints{}, errors.Wrap(err, "query eras reward points error")
	}
	data, err := QueryStorage[StakingEraRewardPoints](c, block, "Staking", "ErasRewardPoints", param)
	if err != nil {
		return data, errors.Wrap(err, "query eras reward points error")
	}
	return data, nil
}

/*
QueryAllNominators lists all active nominator accounts and their preferences.
Accesses "Nominators" storage map under "Staking" module.

Parameters:
  - block: State query block number

Returns:
  - []StakingNominations: Slice of nominator data structures
  - error: Storage access error with context
*/
func (c *Client) QueryAllNominators(block uint32) ([]StakingNominations, error) {
	data, err := QueryStorages[StakingNominations](c, block, "Staking", "Nominators")
	if err != nil {
		return data, errors.Wrap(err, "query all nominators error")
	}
	return data, nil
}

/*
QueryAllBondeds lists all active bonded accounts.
Accesses "Bonded" storage map under "Staking" module.

Parameters:
  - block: State query block number

Returns:
  - []types.AccountID: Slice of account identifier data structures
  - error: Storage access error with context
*/
func (c *Client) QueryAllBondeds(block uint32) ([]types.AccountID, error) {
	data, err := QueryStorages[types.AccountID](c, block, "Staking", "Bonded")
	if err != nil {
		return data, errors.Wrap(err, "query all bondeds error")
	}
	return data, nil
}

/*
QueryValidatorCommission retrieves validator's commission preferences.
Queries "Validators" storage map with account ID parameter.

Parameters:
  - accountId: Validator's account identifier (32-byte array)
  - block: Target block for query

Returns:
  - StakingValidatorPrefs: Commission rate and preferences
  - error: Wrapped storage error
*/
func (c *Client) QueryValidatorCommission(accountId []byte, block uint32) (StakingValidatorPrefs, error) {
	data, err := QueryStorage[StakingValidatorPrefs](c, block, "Staking", "Validators", accountId)
	if err != nil {
		return data, errors.Wrap(err, "query validator commission error")
	}
	return data, nil
}

/*
QueryEraValidatorReward gets validator reward for specific era.
Accesses "ErasValidatorReward" storage with era parameter.

Parameters:
  - era: Reward calculation era
  - block: Historical block number

Returns:
  - types.U128: Era-specific validator reward amount
  - error: Composite error including encoding issues
*/
func (c *Client) QueryEraValidatorReward(era, block uint32) (types.U128, error) {
	param, err := codec.Encode(era)
	if err != nil {
		return types.U128{}, errors.Wrap(err, "query eras validator reward error")
	}
	data, err := QueryStorage[types.U128](c, block, "Staking", "ErasValidatorReward", param)
	if err != nil {
		return data, errors.Wrap(err, "query eras validator reward error")
	}
	return data, nil
}

/*
QueryLedger fetches staking ledger for an account.
Queries "Ledger" storage map with account ID parameter.

Parameters:
  - accountId: Stash account identifier
  - block: Query block number

Returns:
  - StakingLedger: Complete staking ledger information
  - error: Enhanced error with account context
*/
func (c *Client) QueryLedger(accountId []byte, block uint32) (StakingLedger, error) {
	data, err := QueryStorage[StakingLedger](c, block, "Staking", "Ledger", accountId)
	if err != nil {
		return data, errors.Wrap(err, "query ledger error")
	}
	return data, nil
}

/*
QueryErasStakers retrieves validator exposure for specific era.
Combines era parameter and account ID to query "ErasStakers".

Parameters:
  - accountId: Validator account ID
  - era: Target exposure era
  - block: Historical block number

Returns:
  - StakingExposure: Validator's era exposure details
  - error: Wrapped multi-parameter query error
*/
func (c *Client) QueryErasStakers(accountId []byte, era, block uint32) (StakingExposure, error) {

	param, err := codec.Encode(era)
	if err != nil {
		return StakingExposure{}, errors.Wrap(err, "query eras stakers error")
	}
	data, err := QueryStorage[StakingExposure](c, block, "Staking", "ErasStakers", param, accountId)
	if err != nil {
		return data, errors.Wrap(err, "query eras stakers error")
	}
	return data, nil
}

/*
QueryNominators lists all active nominator accounts and their preferences.
Accesses "Nominators" storage map under "Staking" module.

Parameters:
  - block: State query block number

Returns:
  - []StakingNominations: Slice of nominator data structures
  - error: Storage access error with context
*/
func (c *Client) QueryNominators(accountId []byte, block uint32) (StakingNominations, error) {
	data, err := QueryStorage[StakingNominations](c, block, "Staking", "Nominators", accountId)
	if err != nil {
		return data, errors.Wrap(err, "query nominators error")
	}
	return data, nil
}

/*
QueryAllErasStakersPaged gets paginated validator exposure data.
Iterates through 256 pages of "ErasStakersPaged" storage.

Parameters:
  - accountId: Validator account identifier
  - era: Target staking era
  - block: Query block number

Returns:
  - []StakingExposurePaged: Paginated exposure data slices
  - error: Composite error during paged queries
*/
func (c *Client) QueryAllErasStakersPaged(accountId []byte, era, block uint32) ([]StakingExposurePaged, error) {
	param, err := codec.Encode(era)
	if err != nil {
		return nil, errors.Wrap(err, "query all eras stakers paged error")
	}

	data := make([]StakingExposurePaged, 0, 256)

	for i := range 256 {
		paramI, err := codec.Encode(types.U32(i))
		if err != nil {
			return nil, errors.Wrap(err, "query all eras stakers paged error")
		}
		d, err := QueryStorage[StakingExposurePaged](c, block, "Staking", "ErasStakersPaged", param, accountId, paramI)
		if err != nil {
			return nil, errors.Wrap(err, "query all eras stakers paged error")
		}
		data = append(data, d)
	}
	return data, nil
}

func (c *Client) QueryErasStakersOveriew(accountId []byte, era, block uint32) (PagedExposureMetadata, error) {

	param, err := codec.Encode(era)
	if err != nil {
		return PagedExposureMetadata{}, errors.Wrap(err, "query eras stakers overiew error")
	}
	data, err := QueryStorage[PagedExposureMetadata](c, block, "Staking", "ErasStakers", param, accountId)
	if err != nil {
		return data, errors.Wrap(err, "query eras stakers overiew error")
	}
	return data, nil
}
