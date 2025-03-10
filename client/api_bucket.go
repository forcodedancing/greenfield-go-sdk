package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	gnfdsdk "github.com/bnb-chain/greenfield/sdk/types"
	gnfdTypes "github.com/bnb-chain/greenfield/types"
	"github.com/bnb-chain/greenfield/types/s3util"
	permTypes "github.com/bnb-chain/greenfield/x/permission/types"
	storageTypes "github.com/bnb-chain/greenfield/x/storage/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/rs/zerolog/log"

	"github.com/bnb-chain/greenfield-go-sdk/pkg/utils"
	"github.com/bnb-chain/greenfield-go-sdk/types"
)

type Bucket interface {
	GetCreateBucketApproval(ctx context.Context, createBucketMsg *storageTypes.MsgCreateBucket) (*storageTypes.MsgCreateBucket, error)
	// CreateBucket get approval of creating bucket and send createBucket txn to greenfield chain
	// primaryAddr indicates the HEX-encoded string of the primary storage provider address to which the bucket will be created
	CreateBucket(ctx context.Context, bucketName string, primaryAddr string, opts types.CreateBucketOptions) (string, error)
	DeleteBucket(ctx context.Context, bucketName string, opt types.DeleteBucketOption) (string, error)

	UpdateBucketVisibility(ctx context.Context, bucketName string, visibility storageTypes.VisibilityType, opt types.UpdateVisibilityOption) (string, error)
	UpdateBucketInfo(ctx context.Context, bucketName string, opts types.UpdateBucketOption) (string, error)
	UpdateBucketPaymentAddr(ctx context.Context, bucketName string, paymentAddr sdk.AccAddress, opt types.UpdatePaymentOption) (string, error)

	HeadBucket(ctx context.Context, bucketName string) (*storageTypes.BucketInfo, error)
	HeadBucketByID(ctx context.Context, bucketID string) (*storageTypes.BucketInfo, error)

	PutBucketPolicy(ctx context.Context, bucketName string, principalStr types.Principal, statements []*permTypes.Statement, opt types.PutPolicyOption) (string, error)
	// DeleteBucketPolicy delete the bucket policy of the principal
	// principalAddr indicates the HEX-encoded string of the principal address
	DeleteBucketPolicy(ctx context.Context, bucketName string, principalAddr string, opt types.DeletePolicyOption) (string, error)
	// GetBucketPolicy get the bucket policy info of the user specified by principalAddr.
	// principalAddr indicates the HEX-encoded string of the principal address
	GetBucketPolicy(ctx context.Context, bucketName string, principalAddr string) (*permTypes.Policy, error)
	// IsBucketPermissionAllowed check if the permission of bucket is allowed to the user.
	// userAddr indicates the HEX-encoded string of the user address
	IsBucketPermissionAllowed(ctx context.Context, userAddr string, bucketName string, action permTypes.ActionType) (permTypes.Effect, error)

	ListBuckets(ctx context.Context) (types.ListBucketsResult, error)
	ListBucketReadRecord(ctx context.Context, bucketName string, opts types.ListReadRecordOptions) (types.QuotaRecordInfo, error)

	BuyQuotaForBucket(ctx context.Context, bucketName string, targetQuota uint64, opt types.BuyQuotaOption) (string, error)
	GetBucketReadQuota(ctx context.Context, bucketName string) (types.QuotaInfo, error)
}

// GetCreateBucketApproval returns the signature info for the approval of preCreating resources
func (c *client) GetCreateBucketApproval(ctx context.Context, createBucketMsg *storageTypes.MsgCreateBucket) (*storageTypes.MsgCreateBucket, error) {
	unsignedBytes := createBucketMsg.GetSignBytes()

	// set the action type
	urlVal := make(url.Values)
	urlVal["action"] = []string{types.CreateBucketAction}

	reqMeta := requestMeta{
		urlValues:     urlVal,
		urlRelPath:    "get-approval",
		contentSHA256: types.EmptyStringSHA256,
		txnMsg:        hex.EncodeToString(unsignedBytes),
	}

	sendOpt := sendOptions{
		method:     http.MethodGet,
		isAdminApi: true,
	}

	primarySPAddr := createBucketMsg.GetPrimarySpAddress()
	endpoint, err := c.getSPUrlByAddr(primarySPAddr)
	if err != nil {
		log.Error().Msg(fmt.Sprintf("route endpoint by addr: %s failed, err: %s", primarySPAddr, err.Error()))
		return nil, err
	}

	resp, err := c.sendReq(ctx, reqMeta, &sendOpt, endpoint)
	if err != nil {
		return nil, err
	}

	// fetch primary signed msg from sp response
	signedRawMsg := resp.Header.Get(types.HTTPHeaderSignedMsg)
	if signedRawMsg == "" {
		return nil, errors.New("fail to fetch pre createObject signature")
	}

	signedMsgBytes, err := hex.DecodeString(signedRawMsg)
	if err != nil {
		return nil, err
	}

	var signedMsg storageTypes.MsgCreateBucket
	storageTypes.ModuleCdc.MustUnmarshalJSON(signedMsgBytes, &signedMsg)

	return &signedMsg, nil
}

// CreateBucket get approval of creating bucket and send createBucket txn to greenfield chain
func (c *client) CreateBucket(ctx context.Context, bucketName string, primaryAddr string, opts types.CreateBucketOptions) (string, error) {
	address, err := sdk.AccAddressFromHexUnsafe(primaryAddr)
	if err != nil {
		return "", err
	}

	var visibility storageTypes.VisibilityType
	if opts.Visibility == storageTypes.VISIBILITY_TYPE_UNSPECIFIED {
		visibility = storageTypes.VISIBILITY_TYPE_PRIVATE // set default visibility type
	} else {
		visibility = opts.Visibility
	}

	var paymentAddr sdk.AccAddress
	if opts.PaymentAddress != "" {
		paymentAddr, err = sdk.AccAddressFromHexUnsafe(opts.PaymentAddress)
		if err != nil {
			return "", err
		}
	}

	createBucketMsg := storageTypes.NewMsgCreateBucket(c.MustGetDefaultAccount().GetAddress(), bucketName,
		visibility, address, paymentAddr, 0, nil, opts.ChargedQuota)

	err = createBucketMsg.ValidateBasic()
	if err != nil {
		return "", err
	}
	signedMsg, err := c.GetCreateBucketApproval(ctx, createBucketMsg)
	if err != nil {
		return "", err
	}

	// set the default txn broadcast mode as block mode
	if opts.TxOpts == nil {
		broadcastMode := tx.BroadcastMode_BROADCAST_MODE_BLOCK
		opts.TxOpts = &gnfdsdk.TxOption{Mode: &broadcastMode}
	}

	resp, err := c.chainClient.BroadcastTx(ctx, []sdk.Msg{signedMsg}, opts.TxOpts)
	if err != nil {
		return "", err
	}

	return resp.TxResponse.TxHash, err
}

// DeleteBucket send DeleteBucket txn to greenfield chain and return txn hash
func (c *client) DeleteBucket(ctx context.Context, bucketName string, opt types.DeleteBucketOption) (string, error) {
	if err := s3util.CheckValidBucketName(bucketName); err != nil {
		return "", err
	}
	delBucketMsg := storageTypes.NewMsgDeleteBucket(c.MustGetDefaultAccount().GetAddress(), bucketName)
	return c.sendTxn(ctx, delBucketMsg, opt.TxOpts)
}

// UpdateBucketVisibility update the visibilityType of bucket
func (c *client) UpdateBucketVisibility(ctx context.Context, bucketName string,
	visibility storageTypes.VisibilityType, opt types.UpdateVisibilityOption) (string, error) {
	bucketInfo, err := c.HeadBucket(ctx, bucketName)
	if err != nil {
		return "", err
	}

	paymentAddr, err := sdk.AccAddressFromHexUnsafe(bucketInfo.PaymentAddress)
	if err != nil {
		return "", err
	}

	updateBucketMsg := storageTypes.NewMsgUpdateBucketInfo(c.MustGetDefaultAccount().GetAddress(), bucketName, &bucketInfo.ChargedReadQuota, paymentAddr, visibility)
	return c.sendTxn(ctx, updateBucketMsg, opt.TxOpts)
}

// UpdateBucketPaymentAddr  update the payment addr of bucket
func (c *client) UpdateBucketPaymentAddr(ctx context.Context, bucketName string,
	paymentAddr sdk.AccAddress, opt types.UpdatePaymentOption) (string, error) {
	bucketInfo, err := c.HeadBucket(ctx, bucketName)
	if err != nil {
		return "", err
	}

	updateBucketMsg := storageTypes.NewMsgUpdateBucketInfo(c.MustGetDefaultAccount().GetAddress(), bucketName, &bucketInfo.ChargedReadQuota, paymentAddr, bucketInfo.Visibility)
	return c.sendTxn(ctx, updateBucketMsg, opt.TxOpts)
}

// UpdateBucketInfo update the bucket meta on chain, including read quota, payment address or visibility
func (c *client) UpdateBucketInfo(ctx context.Context, bucketName string, opts types.UpdateBucketOption) (string, error) {
	bucketInfo, err := c.HeadBucket(ctx, bucketName)
	if err != nil {
		return "", err
	}

	if opts.Visibility == bucketInfo.Visibility && opts.PaymentAddress == "" && opts.ChargedQuota == nil {
		return "", errors.New("no meta need to update")
	}

	var visibility storageTypes.VisibilityType
	var chargedReadQuota uint64
	var paymentAddr sdk.AccAddress

	if opts.Visibility != bucketInfo.Visibility {
		visibility = opts.Visibility
	} else {
		visibility = bucketInfo.Visibility
	}

	if opts.PaymentAddress != "" {
		paymentAddr, err = sdk.AccAddressFromHexUnsafe(opts.PaymentAddress)
		if err != nil {
			return "", err
		}
	} else {
		paymentAddr, err = sdk.AccAddressFromHexUnsafe(bucketInfo.PaymentAddress)
		if err != nil {
			return "", err
		}
	}

	if opts.ChargedQuota != nil {
		chargedReadQuota = *opts.ChargedQuota
	} else {
		chargedReadQuota = bucketInfo.ChargedReadQuota
	}

	updateBucketMsg := storageTypes.NewMsgUpdateBucketInfo(c.MustGetDefaultAccount().GetAddress(), bucketName,
		&chargedReadQuota, paymentAddr, visibility)

	// set the default txn broadcast mode as block mode
	if opts.TxOpts == nil {
		broadcastMode := tx.BroadcastMode_BROADCAST_MODE_BLOCK
		opts.TxOpts = &gnfdsdk.TxOption{Mode: &broadcastMode}
	}

	return c.sendTxn(ctx, updateBucketMsg, opts.TxOpts)
}

// HeadBucket query the bucketInfo on chain, return the bucket info if exists
// return err info if bucket not exist
func (c *client) HeadBucket(ctx context.Context, bucketName string) (*storageTypes.BucketInfo, error) {
	queryHeadBucketRequest := storageTypes.QueryHeadBucketRequest{
		BucketName: bucketName,
	}
	queryHeadBucketResponse, err := c.chainClient.HeadBucket(ctx, &queryHeadBucketRequest)
	if err != nil {
		return nil, err
	}

	return queryHeadBucketResponse.BucketInfo, nil
}

// HeadBucketByID query the bucketInfo on chain by bucketId, return the bucket info if exists
// return err info if bucket not exist
func (c *client) HeadBucketByID(ctx context.Context, bucketID string) (*storageTypes.BucketInfo, error) {
	headBucketRequest := &storageTypes.QueryHeadBucketByIdRequest{
		BucketId: bucketID,
	}

	headBucketResponse, err := c.chainClient.HeadBucketById(ctx, headBucketRequest)
	if err != nil {
		return nil, err
	}

	return headBucketResponse.BucketInfo, nil
}

// PutBucketPolicy apply bucket policy to the principal, return the txn hash
func (c *client) PutBucketPolicy(ctx context.Context, bucketName string, principalStr types.Principal,
	statements []*permTypes.Statement, opt types.PutPolicyOption) (string, error) {
	resource := gnfdTypes.NewBucketGRN(bucketName)
	principal := &permTypes.Principal{}
	if err := principal.Unmarshal([]byte(principalStr)); err != nil {
		return "", err
	}

	putPolicyMsg := storageTypes.NewMsgPutPolicy(c.MustGetDefaultAccount().GetAddress(), resource.String(),
		principal, statements, opt.PolicyExpireTime)

	return c.sendPutPolicyTxn(ctx, putPolicyMsg, opt.TxOpts)
}

// DeleteBucketPolicy delete the bucket policy of the principal
func (c *client) DeleteBucketPolicy(ctx context.Context, bucketName string, principalAddr string, opt types.DeletePolicyOption) (string, error) {
	resource := gnfdTypes.NewBucketGRN(bucketName).String()
	addr, err := sdk.AccAddressFromHexUnsafe(principalAddr)
	if err != nil {
		return "", err
	}

	principal := permTypes.NewPrincipalWithAccount(addr)

	return c.sendDelPolicyTxn(ctx, c.MustGetDefaultAccount().GetAddress(), resource, principal, opt.TxOpts)
}

// IsBucketPermissionAllowed check if the permission of bucket is allowed to the user.
func (c *client) IsBucketPermissionAllowed(ctx context.Context, userAddr string,
	bucketName string, action permTypes.ActionType) (permTypes.Effect, error) {
	_, err := sdk.AccAddressFromHexUnsafe(userAddr)
	if err != nil {
		return permTypes.EFFECT_DENY, err
	}
	verifyReq := storageTypes.QueryVerifyPermissionRequest{
		Operator:   userAddr,
		BucketName: bucketName,
		ActionType: action,
	}

	verifyResp, err := c.chainClient.VerifyPermission(ctx, &verifyReq)
	if err != nil {
		return permTypes.EFFECT_DENY, err
	}

	return verifyResp.Effect, nil
}

// GetBucketPolicy get the bucket policy info of the user specified by principalAddr.
func (c *client) GetBucketPolicy(ctx context.Context, bucketName string, principalAddr string) (*permTypes.Policy, error) {
	_, err := sdk.AccAddressFromHexUnsafe(principalAddr)
	if err != nil {
		return nil, err
	}

	resource := gnfdTypes.NewBucketGRN(bucketName).String()
	queryPolicy := storageTypes.QueryPolicyForAccountRequest{
		Resource:         resource,
		PrincipalAddress: principalAddr,
	}

	queryPolicyResp, err := c.chainClient.QueryPolicyForAccount(ctx, &queryPolicy)
	if err != nil {
		return nil, err
	}

	return queryPolicyResp.Policy, nil
}

// ListBuckets list buckets for the owner
func (c *client) ListBuckets(ctx context.Context) (types.ListBucketsResult, error) {
	reqMeta := requestMeta{
		contentSHA256: types.EmptyStringSHA256,
		userAddress:   c.MustGetDefaultAccount().GetAddress().String(),
	}

	sendOpt := sendOptions{
		method:           http.MethodGet,
		disableCloseBody: true,
	}

	endpoint, err := c.getInServiceSP()
	if err != nil {
		log.Error().Msg(fmt.Sprintf("get in-service SP fail %s", err.Error()))
		return types.ListBucketsResult{}, err
	}

	resp, err := c.sendReq(ctx, reqMeta, &sendOpt, endpoint)
	if err != nil {
		log.Error().Msg("the list of user's buckets failed: " + err.Error())
		return types.ListBucketsResult{}, err
	}
	defer utils.CloseResponse(resp)

	listBucketsResult := types.ListBucketsResult{}
	//unmarshal the json content from response body
	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		log.Error().Msg("the list of user's buckets failed: " + err.Error())
		return types.ListBucketsResult{}, err
	}

	bufStr := buf.String()
	err = json.Unmarshal([]byte(bufStr), &listBucketsResult)

	//TODO(annie) remove tolerance for unmarshal err after structs got stabilized
	if err != nil && listBucketsResult.Buckets == nil {
		return types.ListBucketsResult{}, err
	}

	return listBucketsResult, nil
}

// ListBucketReadRecord returns the read record of this month, the return items should be no more than maxRecords
// ListReadRecordOption indicates the start timestamp of return read records
func (c *client) ListBucketReadRecord(ctx context.Context, bucketName string, opts types.ListReadRecordOptions) (types.QuotaRecordInfo, error) {
	if err := s3util.CheckValidBucketName(bucketName); err != nil {
		return types.QuotaRecordInfo{}, err
	}
	timeNow := time.Now()
	timeToday := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), 0, 0, 0, 0, timeNow.Location())
	if opts.StartTimeStamp < 0 {
		return types.QuotaRecordInfo{}, errors.New("start timestamp  less than 0")
	}
	var startTimeStamp int64
	if opts.StartTimeStamp == 0 {
		// the timestamp of the first day of this month
		startTimeStamp = timeToday.AddDate(0, 0, -timeToday.Day()+1).UnixMicro()
	} else {
		startTimeStamp = opts.StartTimeStamp
	}
	// the timestamp of the last day of this month
	timeMonthEnd := timeToday.AddDate(0, 1, -timeToday.Day()+1).UnixMicro()

	if timeMonthEnd < startTimeStamp {
		return types.QuotaRecordInfo{}, errors.New("start timestamp larger than the end timestamp of this month")
	}

	params := url.Values{}
	params.Set("list-read-record", "")
	if opts.MaxRecords > 0 {
		params.Set("max-records", strconv.Itoa(opts.MaxRecords))
	} else {
		params.Set("max-records", strconv.Itoa(math.MaxUint32))
	}

	params.Set("start-timestamp", strconv.FormatInt(startTimeStamp, 10))
	params.Set("end-timestamp", strconv.FormatInt(timeMonthEnd, 10))

	reqMeta := requestMeta{
		urlValues:     params,
		bucketName:    bucketName,
		contentSHA256: types.EmptyStringSHA256,
	}

	sendOpt := sendOptions{
		method:           http.MethodGet,
		disableCloseBody: true,
	}

	endpoint, err := c.getSPUrlByBucket(bucketName)
	if err != nil {
		log.Error().Msg(fmt.Sprintf("route endpoint by bucket: %s failed, err: %s", bucketName, err.Error()))
		return types.QuotaRecordInfo{}, err
	}

	resp, err := c.sendReq(ctx, reqMeta, &sendOpt, endpoint)
	if err != nil {
		return types.QuotaRecordInfo{}, err
	}
	defer utils.CloseResponse(resp)

	QuotaRecords := types.QuotaRecordInfo{}
	// decode the xml content from response body
	err = xml.NewDecoder(resp.Body).Decode(&QuotaRecords)
	if err != nil {
		return types.QuotaRecordInfo{}, err
	}

	return QuotaRecords, nil
}

// GetBucketReadQuota return quota info of bucket of current month, include chain quota, free quota and consumed quota
func (c *client) GetBucketReadQuota(ctx context.Context, bucketName string) (types.QuotaInfo, error) {
	if err := s3util.CheckValidBucketName(bucketName); err != nil {
		return types.QuotaInfo{}, err
	}

	year, month, _ := time.Now().Date()
	var date string
	if int(month) < 10 {
		date = strconv.Itoa(year) + "-" + "0" + strconv.Itoa(int(month))
	} else {
		date = strconv.Itoa(year) + "-" + strconv.Itoa(int(month))
	}

	params := url.Values{}
	params.Add("read-quota", "")
	params.Add("year-month", date)

	reqMeta := requestMeta{
		urlValues:     params,
		bucketName:    bucketName,
		contentSHA256: types.EmptyStringSHA256,
	}

	sendOpt := sendOptions{
		method:           http.MethodGet,
		disableCloseBody: true,
	}

	endpoint, err := c.getSPUrlByBucket(bucketName)
	if err != nil {
		log.Error().Msg(fmt.Sprintf("route endpoint by bucket: %s failed, err: %s", bucketName, err.Error()))
		return types.QuotaInfo{}, err
	}

	resp, err := c.sendReq(ctx, reqMeta, &sendOpt, endpoint)
	if err != nil {
		return types.QuotaInfo{}, err
	}
	defer utils.CloseResponse(resp)

	QuotaResult := types.QuotaInfo{}
	// decode the xml content from response body
	err = xml.NewDecoder(resp.Body).Decode(&QuotaResult)
	if err != nil {
		return types.QuotaInfo{}, err
	}

	return QuotaResult, nil
}

// BuyQuotaForBucket buy the target quota of the specific bucket
// targetQuota indicates the target quota to set for the bucket
func (c *client) BuyQuotaForBucket(ctx context.Context, bucketName string, targetQuota uint64, opt types.BuyQuotaOption) (string, error) {
	bucketInfo, err := c.HeadBucket(ctx, bucketName)
	if err != nil {
		return "", err
	}

	paymentAddr, err := sdk.AccAddressFromHexUnsafe(bucketInfo.PaymentAddress)
	if err != nil {
		return "", err
	}
	updateBucketMsg := storageTypes.NewMsgUpdateBucketInfo(c.MustGetDefaultAccount().GetAddress(), bucketName, &targetQuota, paymentAddr, bucketInfo.Visibility)

	resp, err := c.chainClient.BroadcastTx(ctx, []sdk.Msg{updateBucketMsg}, opt.TxOpts)
	if err != nil {
		return "", err
	}

	return resp.TxResponse.TxHash, err
}
