package gnfdclient

import (
	"context"

	"github.com/bnb-chain/greenfield-go-sdk/client/sp"
	"github.com/bnb-chain/greenfield-go-sdk/types"
	storage_type "github.com/bnb-chain/greenfield/x/storage/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog/log"
)

type BucketMeta struct {
	bucketName       string
	isPublic         bool
	creator          sdk.AccAddress
	primarySPAddress sdk.AccAddress
	paymentAddress   sdk.AccAddress
}

// CreateBucket get approval of creating bucket and send createBucket txn to greenfield chain
func (c *GreenfieldClient) CreateBucket(ctx context.Context, bucketMeta BucketMeta,
	timeoutHeight uint64, txOpts types.TxOption, authInfo sp.AuthInfo) error {
	// get approval of creating bucket from sp
	signature, err := c.SPClient.GetApproval(ctx, bucketMeta.bucketName, "", authInfo)
	if err != nil {
		return err
	}

	log.Info().Msg("get approve from sp finish,signature is:" + signature)
	// call chain sdk to send a createBucket txn to greenfield with signature
	createBucketMsg := storage_type.NewMsgCreateBucket(bucketMeta.creator, bucketMeta.bucketName, bucketMeta.isPublic,
		bucketMeta.primarySPAddress, bucketMeta.paymentAddress, timeoutHeight, []byte(signature))

	_, err = c.ChainClient.BroadcastTx([]sdk.Msg{createBucketMsg}, &txOpts)
	if err != nil {
		return nil
	}

	return nil
}
