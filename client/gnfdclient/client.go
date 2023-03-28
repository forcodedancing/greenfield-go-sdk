package gnfdclient

import (
	"context"

	client "github.com/bnb-chain/greenfield-go-sdk/lib/chain"
	"github.com/bnb-chain/greenfield-go-sdk/lib/sp"
	chain "github.com/bnb-chain/greenfield/sdk/client"
	"github.com/bnb-chain/greenfield/sdk/keys"
)

type (
	ChainClient            = *chain.GreenfieldClient
	GreenfieldClientOption = client.GreenfieldClientOption
)

// Client integrates the chain and spHandler
type Client struct {
	chain     ChainClient
	spHandler *sp.SPHandler
	SPInfo    map[string]Endpoint //map of chain address and endpoint
}

type NewClientOption struct {
	SPEndpoint *string
	secure     bool
	km         keys.KeyManager
}

// NewClient returns Client from chain info and sp info
// km pass a keyManager for SP client to sign http request
func NewClient(grpcAddrs string, chainId string, opt NewClientOption, gnfdopts ...GreenfieldClientOption) (*Client, error) {
	chainClient := chain.NewGreenfieldClient(grpcAddrs, chainId, gnfdopts...)

	var spClient *sp.SPHandler
	var err error
	if opt.SPEndpoint != nil {
		spClient, err = sp.NewSpHandler(*opt.SPEndpoint, sp.WithKeyManager(opt.km), sp.WithSecure(opt.secure))
		if err != nil {
			return nil, err
		}
	}
	client := &Client{
		chain:     chainClient,
		spHandler: spClient,
	}

	ctx := context.Background()
	spInfo, err := client.GetSPInfo(ctx)
	if err != nil {
		return nil, err
	}

	client.SPInfo = spInfo
	return client, nil
}
