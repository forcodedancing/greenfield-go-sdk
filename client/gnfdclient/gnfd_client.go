package gnfdclient

import (
	chain "github.com/bnb-chain/greenfield-go-sdk/client/chain"
	sp "github.com/bnb-chain/greenfield-go-sdk/client/sp"
	"github.com/bnb-chain/greenfield-go-sdk/keys"
)

// GreenfieldClient integrate the chainClient and SPClient
type GreenfieldClient struct {
	ChainClient *chain.ChainClient
	SPClient    *sp.SPClient
}

type ChainClientInfo struct {
	rpcAddr  string
	grpcAddr string
}

type SPClientInfo struct {
	endpoint string
	opt      *sp.Option
}

func NewGreenfieldClient(chainInfo ChainClientInfo, spInfo SPClientInfo) (*GreenfieldClient, error) {
	var err error
	spClient := &sp.SPClient{}
	if spInfo.endpoint != "" {
		if spInfo.opt == nil {
			spClient, err = sp.NewSpClient(spInfo.endpoint, &sp.Option{})
			if err != nil {
				return nil, err
			}
		} else {
			spClient, err = sp.NewSpClient(spInfo.endpoint, spInfo.opt)
			if err != nil {
				return nil, err
			}
		}
	}

	chainClientPtr := &chain.ChainClient{}
	if chainInfo.rpcAddr != "" && chainInfo.grpcAddr != "" {
		chainClient := chain.NewChainClient(chainInfo.rpcAddr, chainInfo.grpcAddr)
		chainClientPtr = &chainClient
	}

	return &GreenfieldClient{
		ChainClient: chainClientPtr,
		SPClient:    spClient,
	}, nil
}

func NewGreenfieldClientWithKeyManager(chainInfo ChainClientInfo, spInfo SPClientInfo,
	keyManager keys.KeyManager) (*GreenfieldClient, error) {
	GreenfieldClient, err := NewGreenfieldClient(chainInfo, spInfo)
	if err != nil {
		return nil, err
	}

	GreenfieldClient.ChainClient.SetKeyManager(keyManager)
	err = GreenfieldClient.SPClient.SetKeyManager(keyManager)
	if err != nil {
		return nil, err
	}
	return GreenfieldClient, nil
}
