package main

import (
	. "github.com/MediConCenHK/go-chaincode-common"
	. "github.com/davidkhala/fabric-common-chaincode-golang"
	. "github.com/davidkhala/fabric-common-chaincode-golang/cid"
	. "github.com/davidkhala/goutils"
	Logger "github.com/davidkhala/goutils/logger"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-protos-go/peer"
	"go.uber.org/zap"
)

type GlobalChaincode struct {
	CommonChaincode
}

var logger = Logger.Zap()

func (t GlobalChaincode) Init(stub shim.ChaincodeStubInterface) (response peer.Response) {
	defer Deferred(DeferHandlerPeerResponse, &response)
	t.Prepare(stub)
	logger.Info("Init")
	return shim.Success(nil)
}
func (t GlobalChaincode) putToken(cid ClientIdentity, tokenID string, tokenData TokenData) {
	tokenData.Client = cid
	t.PutStateObj(tokenID, tokenData)
}
func (t GlobalChaincode) getToken(token string) *TokenData {
	var tokenData TokenData
	var exist = t.GetStateObj(token, &tokenData)
	if !exist {
		return nil
	}
	return &tokenData
}
func (t GlobalChaincode) history(token string) []byte {
	var history = ParseHistory(t.GetHistoryForKey(token), nil)
	return ToJson(history)

}

func (t GlobalChaincode) Invoke(stub shim.ChaincodeStubInterface) (response peer.Response) {
	defer Deferred(DeferHandlerPeerResponse, &response)
	t.Prepare(stub)

	var fcn, params = stub.GetFunctionAndParameters()
	logger.Info("Invoke", zap.String("fcn", fcn))
	logger.Debug("Invoke", zap.Strings("params", params))
	var clientID = NewClientIdentity(stub)
	var MspID = clientID.MspID
	var responseBytes []byte
	var tokenRaw = params[0]
	if tokenRaw == "" {
		panicEcosystem("token", "param:token is empty")
	}
	var tokenID = Hash([]byte(tokenRaw))

	var tokenData TokenData
	var time TimeLong
	switch fcn {
	case FcnCreateToken:
		var createRequest TokenCreateRequest
		FromJson([]byte(params[1]), &createRequest)
		var tokenDataPtr = t.getToken(tokenID)
		if tokenDataPtr != nil {
			panicEcosystem("token", "token["+tokenRaw+"] already exist")
		}
		tokenData = createRequest.Build()
		tokenData.OwnerType = OwnerTypeMember
		tokenData.TransferDate = TimeLong(0)
		tokenData.Issuer = MspID
		tokenData.Manager = MspID
		tokenData.IssuerClient = clientID
		t.putToken(clientID, tokenID, tokenData)

	case FcnGetToken:
		var tokenDataPtr = t.getToken(tokenID)
		if tokenDataPtr == nil {
			break
		}
		responseBytes = ToJson(*tokenDataPtr)
	case FcnRenewToken:
		var newExpiryTime = time.FromString(params[1])
		var tokenDataPtr = t.getToken(tokenID)
		if tokenDataPtr == nil {
			panicEcosystem("token", "token["+tokenRaw+"] not found")
		}
		tokenData = *tokenDataPtr
		tokenData.ExpiryDate = newExpiryTime
		t.putToken(clientID, tokenID, tokenData)
	case FcnTokenHistory:
		responseBytes = t.history(tokenID)
	case FcnDeleteToken:
		var tokenDataPtr = t.getToken(tokenID)
		if tokenDataPtr == nil {
			break // not exist, swallow
		}
		tokenData = *tokenDataPtr
		if MspID != tokenData.Manager {
			panicEcosystem("CID", "["+tokenRaw+"]Token Data Manager("+tokenData.Manager+") mismatched with tx creator MspID: "+MspID)
		}
		t.DelState(tokenID)
	case FcnMoveToken:
		var transferReq TokenTransferRequest

		FromJson([]byte(params[1]), &transferReq)

		var tokenDataPtr = t.getToken(tokenID)
		if tokenDataPtr == nil {
			panicEcosystem("token", "token["+tokenRaw+"] not found")
		}
		tokenData = *tokenDataPtr
		if tokenData.OwnerType != OwnerTypeMember {
			panicEcosystem("OwnerType", "original token OwnerType should be member, but got "+tokenData.OwnerType.To())
		}
		if tokenData.TransferDate != TimeLong(0) {
			panicEcosystem("token", "token["+tokenRaw+"] was transferred")
		}

		tokenData = transferReq.ApplyOn(tokenData)
		tokenData.Manager = MspID
		tokenData.OwnerType = OwnerTypeNetwork
		tokenData.TransferDate = time.FromTimeStamp(t.GetTxTimestamp())
		t.putToken(clientID, tokenID, tokenData)
	default:
		panicEcosystem("unknown", "unknown fcn:"+fcn)
	}
	logger.Debug(string(responseBytes))
	return shim.Success(responseBytes)
}

func main() {
	var cc = GlobalChaincode{}
	ChaincodeStart(cc)
}
