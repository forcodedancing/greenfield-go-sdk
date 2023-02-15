package sp

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/bnb-chain/greenfield-go-sdk/utils"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const ChallengeUrl = "challenge"

// GetApproval return the signature info for the approval of preCreating resources
func (c *SPClient) GetApproval(ctx context.Context, bucketName, objectName string, authInfo AuthInfo) (string, error) {
	if err := utils.IsValidBucketName(bucketName); err != nil {
		return "", err
	}

	if objectName != "" {
		if err := utils.IsValidObjectName(objectName); err != nil {
			return "", err
		}
	}

	// set the action type
	urlVal := make(url.Values)
	if objectName != "" {
		urlVal["action"] = []string{CreateObjectAction}
	} else {
		urlVal["action"] = []string{CreateBucketAction}
	}

	reqMeta := requestMeta{
		bucketName:    bucketName,
		objectName:    objectName,
		urlValues:     urlVal,
		urlRelPath:    "get-approval",
		contentSHA256: EmptyStringSHA256,
	}

	sendOpt := sendOptions{
		method:     http.MethodGet,
		isAdminApi: true,
	}

	resp, err := c.sendReq(ctx, reqMeta, &sendOpt, authInfo)
	if err != nil {
		log.Printf("get approval rejected: %s \n", err.Error())
		return "", err
	}

	// fetch primary sp signature from sp response
	signature := resp.Header.Get(HTTPHeaderPreSignature)
	if signature == "" {
		return "", errors.New("fail to fetch pre createObject signature")
	}

	return signature, nil
}

// ChallengeInfo indicates the challenge object info
type ChallengeInfo struct {
	ObjectId   string
	PieceIndex int
	SPAddr     sdk.AccAddress // the sp address which to be challenge
}

// ChallengeResult indicates the challenge hash and data results
type ChallengeResult struct {
	PieceData     io.ReadCloser
	IntegrityHash string
	PiecesHash    []string
}

// ChallengeSP send request to challenge and get challenge result info
func (c *SPClient) ChallengeSP(ctx context.Context, info ChallengeInfo, authInfo AuthInfo) (ChallengeResult, error) {
	if info.ObjectId == "" {
		return ChallengeResult{}, errors.New("fail to get objectId")
	}

	if info.PieceIndex < 0 || info.PieceIndex > EncodeShards {
		return ChallengeResult{}, errors.New("index error, should be 0 to 5")
	}

	if info.SPAddr == nil {
		return ChallengeResult{}, errors.New("challenge sp addr is nil")
	}

	reqMeta := requestMeta{
		urlRelPath:    ChallengeUrl,
		contentSHA256: EmptyStringSHA256,
		challengeInfo: info,
	}

	sendOpt := sendOptions{
		method:     http.MethodGet,
		isAdminApi: true,
	}

	resp, err := c.sendReq(ctx, reqMeta, &sendOpt, authInfo)
	if err != nil {
		log.Printf("get challenge result fail: %s \n", err.Error())
		return ChallengeResult{}, err
	}

	// fetch integrity hash
	integrityHash := resp.Header.Get(HTTPHeaderIntegrityHash)
	// fetch piece hashes
	pieceHashs := resp.Header.Get(HTTPHeaderPieceHash)

	if integrityHash == "" || pieceHashs == "" {
		utils.CloseResponse(resp)
		return ChallengeResult{}, errors.New("fail to fetch hash info")
	}

	hashList := strings.Split(pieceHashs, ",")
	if len(hashList) <= 1 {
		return ChallengeResult{}, errors.New("get piece hashes less than 1")
	}

	result := ChallengeResult{
		PieceData:     resp.Body,
		IntegrityHash: integrityHash,
		PiecesHash:    hashList,
	}

	return result, nil
}
