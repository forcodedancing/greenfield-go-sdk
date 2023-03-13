package chain

import (
	"github.com/bnb-chain/greenfield-go-sdk/keys"
	"github.com/bnb-chain/greenfield/sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
	"time"
)

const (
	Relayer0Privkey = "31d8f71a3f631bb90fc980a7bb631a7314b8bf9e2c78f3833f7afb9f1d91884d"

	Validator0PrivKey = "a5ae825a4d0f6e7ddea8823b76fba0357b9c31d6a9965bc9df00300bd3445bad"
	Validator1PrivKey = "763b29b5e72461c1228850d97e15c324e474cd220cd1362935276c4647ab1f34"
	Validator2PrivKey = "0d7f1c4be7d77be7d0858e383fdf37ed882fbe461ae2c79f04935f73bf904cee"
	Validator3PrivKey = "326dc94ff32fd363bd416fe5a6595a3b27cb670e1b30a8bd3dfc311c5ec17989"
	Validator4PrivKey = "668ad956a051bf16b521c29c094fb18b73a5e917475e903c076b6634d35035bb"

	GNFD_RPC  = "https://gnfd-dev.bnbchain.world:443"
	GNFD_GRPC = "gnfd-grpc-plaintext-dev.bnbchain.world:9090"
	Chain_ID  = "greenfield_9000-1741"

	GNFD_Realyer1_ADDRESS = "0xA4A2957E858529FFABBBb483D1D704378a9fca6b"
)

func TestCrossChainUpgrade(t *testing.T) {
	km, _ := keys.NewPrivateKeyManager(Validator0PrivKey)
	client := client.NewGreenfieldClient(GNFD_GRPC, Chain_ID, client.WithKeyManager(km),
		client.WithGrpcDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	var paramChanges []paramstypes.ParamChange

	ug1 := paramstypes.ParamChange{
		Subspace: "BSC",
		Key:      "upgrade",
		Value:    "0x1c85638e118b37167e9298c2268758e058DdfDA0",
	}
	paramChanges = append(paramChanges, ug1)

	ug2 := paramstypes.ParamChange{
		Subspace: "BSC",
		Key:      "upgrade",
		Value:    "0x367761085BF3C12e5DA2Df99AC6E1a824612b8fb",
	}
	paramChanges = append(paramChanges, ug2)

	from := km.GetAddr().String()
	content := paramstypes.NewCrossChainParameterChangeProposal(
		"upgrade gov and crossChain",
		"upgrade gov and crossChain",
		paramChanges,
		[]string{"0xA43C8fA0cb6567312091fb14ebf4d0f65De4a6E4", "0x39c3A55F68Bf9f2992776991F25Aac6813a4F1d0"},
	)
	InitDeposit, err := sdk.ParseCoinsNormalized("1000000000000000001BNB")
	assert.NoError(t, err)

	govAcctAddress := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	contentMsg, err := govv1.NewLegacyContent(content, govAcctAddress)
	assert.NoError(t, err)
	msg, err := govv1.NewMsgSubmitProposal([]sdk.Msg{contentMsg}, InitDeposit, from, "")
	assert.NoError(t, err)
	tx, err := client.BroadcastTx([]sdk.Msg{msg}, nil)
	assert.NoError(t, err)

	t.Log(tx.TxResponse.String())
	t.Log(tx.TxResponse.TxHash)
}

func TestVoteProposal(t *testing.T) {

	proposalId := uint64(15)

	err := vote(Validator1PrivKey, proposalId)
	assert.NoError(t, err)
	time.Sleep(10 * time.Second)

	err = vote(Validator2PrivKey, proposalId)
	assert.NoError(t, err)
	time.Sleep(10 * time.Second)

	err = vote(Validator3PrivKey, proposalId)
	assert.NoError(t, err)

	err = vote(Validator4PrivKey, proposalId)
	assert.NoError(t, err)
}

func vote(key string, proposalId uint64) error {
	km, _ := keys.NewPrivateKeyManager(key)
	client := client.NewGreenfieldClient(GNFD_GRPC, Chain_ID, client.WithKeyManager(km),
		client.WithGrpcDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	vote := govv1.MsgVote{
		ProposalId: proposalId,
		Voter:      km.GetAddr().String(),
		Option:     govv1.VoteOption_VOTE_OPTION_YES,
		Metadata:   "",
	}
	_, err := client.BroadcastTx([]sdk.Msg{&vote}, nil)
	if err != nil {
		return err
	}
	return nil
}
