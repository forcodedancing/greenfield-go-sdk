package bank

import (
	"context"
	gnfdclient "github.com/bnb-chain/greenfield-go-sdk/client/chain"
	"github.com/bnb-chain/greenfield-go-sdk/client/test"
	oracletypes "github.com/cosmos/cosmos-sdk/x/oracle/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestOracleParams(t *testing.T) {
	client := gnfdclient.NewChainClient(test.TEST_GRPC_ADDR, test.TEST_CHAIN_ID)

	query := oracletypes.QueryParamsRequest{}
	res, err := client.OracleQueryClient.Params(context.Background(), &query)
	assert.NoError(t, err)

	t.Log(res.GetParams())
}
