package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
)

const blockHeightQueryParam = "block_height"

// getAccount handler retrieves account by address and returns the response
func getAccount(r *request, backend access.API, link LinkGenerator) (interface{}, error) {
	address, err := toAddress(r.getVar("address"))
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	// retrieve account and decide based on provided query params which rpc method to envoke
	var account *flow.Account
	height := r.getQueryParam(blockHeightQueryParam)
	switch height {
	case sealedHeightQueryParam:
		account, err = backend.GetAccountAtLatestBlock(r.Context(), address)
		if err != nil {
			return nil, err
		}
	case finalHeightQueryParam:
		// GetAccountAtLatestBlock only lookup account at the latest sealed block, hence lookup finalized block using
		// the GetLatestBlockHeader call first to find latest finalized height
		finalizedBlkHeader, err := backend.GetLatestBlockHeader(r.Context(), false)
		if err != nil {
			return nil, err
		}
		account, err = backend.GetAccountAtBlockHeight(r.Context(), address, finalizedBlkHeader.Height)
		if err != nil {
			return nil, err
		}
	default: // by default, we get an account at the latest block
		h, err := toHeight(height)
		if err != nil {
			return nil, NewBadRequestError(err)
		}
		account, err = backend.GetAccountAtBlockHeight(r.Context(), address, h)
		if err != nil {
			return nil, err
		}
	}

	return accountResponse(account, link, r.expandFields)
}
