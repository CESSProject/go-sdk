package chain

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/pkg/errors"
)

/*
QueryValidators retrieves active validator accounts for the given block.
Accesses "Validators" storage entry under "Session" module which contains
the current authority set.

Parameters:
  - block: Block number for state query

Returns:
  - []types.AccountID: Slice of validator account identifiers
  - error: Wrapped storage access error with context
*/
func (c *Client) QueryValidators(block uint32) ([]types.AccountID, error) {
	data, err := QueryStorage[[]types.AccountID](c, block, "Session", "Validators")
	if err != nil {
		return data, errors.Wrap(err, "query validators error")
	}
	return data, nil
}
