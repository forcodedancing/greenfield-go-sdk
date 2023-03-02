package chain

import (
	"github.com/bnb-chain/greenfield-go-sdk/client/test"
	"github.com/bnb-chain/greenfield-go-sdk/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
)

func TestCrossChainParametersChangeProposalV1Beta1(t *testing.T) {
	km, _ := keys.NewPrivateKeyManager(test.TEST_PRIVATE_KEY)
	client := NewGreenfieldClient(test.TEST_GRPC_ADDR, test.TEST_CHAIN_ID, WithKeyManager(km),
		WithGrpcDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	var paramChanges []paramstypes.ParamChange
	paramChanges = append(paramChanges,
		paramstypes.ParamChange{
			Subspace: "BSC",
			Key:      "batchSizeForOracle",
			Value:    "0000000000000000000000000000000000000000000000000000000000000033", // 51
		})
	content := paramstypes.NewCrossChainParameterChangeProposal(
		"change smart contract params",
		"change smart contract params",
		paramChanges,
		[]string{"0xa4AFDb1598A672E4F6eD4E4b753b007d7b04d496"},
	)
	InitDeposit, _ := sdk.ParseCoinsNormalized("1000000000000000001BNB")
	govAcctAddress := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	contentMsg, _ := govv1.NewLegacyContent(content, govAcctAddress)
	msg, _ := govv1.NewMsgSubmitProposal([]sdk.Msg{contentMsg}, InitDeposit, km.GetAddr().String(), "")
	client.BroadcastTx([]sdk.Msg{msg}, nil)
}

func TestCrossChainUpgrade(t *testing.T) {
	km, _ := keys.NewPrivateKeyManager(test.TEST_PRIVATE_KEY)
	client := NewGreenfieldClient(test.TEST_GRPC_ADDR, test.TEST_CHAIN_ID, WithKeyManager(km),
		WithGrpcDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	var paramChanges []paramstypes.ParamChange

	ug1 := paramstypes.ParamChange{
		Subspace: "BSC",
		Key:      "upgrade",
		Value:    "0x1fA02b2d6A771842690194Cf62D91bdd92BfE28d", // value
	}
	paramChanges = append(paramChanges, ug1)

	ug2 := paramstypes.ParamChange{
		Subspace: "BSC",
		Key:      "upgrade",
		Value:    "0xdbC43Ba45381e02825b14322cDdd15eC4B3164E6",
	}
	paramChanges = append(paramChanges, ug2)

	from := km.GetAddr().String()
	content := paramstypes.NewCrossChainParameterChangeProposal(
		"upgrade gov and crossChain",
		"upgrade gov and crossChain",
		paramChanges,
		[]string{"0x9E02f3a8567587D27d7EB1D087408D062b4c6a1c", "0xa4AFDb1598A672E4F6eD4E4b753b007d7b04d496"}, // target
	)
	InitDeposit, _ := sdk.ParseCoinsNormalized("1000000000000000001BNB")
	govAcctAddress := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	contentMsg, _ := govv1.NewLegacyContent(content, govAcctAddress)
	msg, _ := govv1.NewMsgSubmitProposal([]sdk.Msg{contentMsg}, InitDeposit, from, "")
	client.BroadcastTx([]sdk.Msg{msg}, nil)

}

func TestVoteProposal(t *testing.T) {
	vKey0 := "78b42a1703f497fabf74163f1c814a6205ff96ea0ad9eb33169d087db293d9f7"
	proposalId := uint64(5)
	vote(vKey0, proposalId)
}

func vote(key string, proposalId uint64) error {
	km, _ := keys.NewPrivateKeyManager(key)
	client := NewGreenfieldClient(test.TEST_GRPC_ADDR, test.TEST_CHAIN_ID, WithKeyManager(km),
		WithGrpcDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
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
