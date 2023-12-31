package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-protos-go/peer"
)

// SimpleAsset ...
type SimpleAsset struct {
	Content string `json:"content"`
	TxID    string `json:"txID"`
}

// SimpleAssetChaincode ...
type SimpleAssetChaincode struct {
}

// Init ...
func (cc *SimpleAssetChaincode) Init(stub shim.ChaincodeStubInterface) peer.Response {
	return shim.Success(nil)
}

// Invoke ...
func (cc *SimpleAssetChaincode) Invoke(stub shim.ChaincodeStubInterface) peer.Response {
	var (
		err    error
		result string
	)

	function, args := stub.GetFunctionAndParameters()

	switch function {
	case "init":
		return cc.Init(stub)
	case "query":
		result, err = query(stub, args)
	case "queryPrivateData":
		result, err = queryPrivateData(stub, args)
	case "store":
		result, err = store(stub, args)
	case "storePrivateData":
		result, err = storePrivateData(stub, args)
	case "setEvent":
		result, err = setEvent(stub, args)
	default:
		return shim.Error("Unknown function")
	}

	if err != nil {
		return shim.Error(fmt.Sprintf("[%s] with args (%s) failed: %s", function, strings.Join(args, " | "), err.Error()))
	}

	return shim.Success([]byte(result))

}

func query(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("Incorrect arguments. Expecting a key")
	}

	value, err := stub.GetState(args[0])
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve value of the specified key '%s': %w", args[0], err)
	}

	// asset not found
	if value == nil || len(value) == 0 {
		return "", nil
	}

	return string(value), nil
}

func queryPrivateData(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("Incorrect arguments. Expecting a key")
	}

	value, err := stub.GetPrivateData("dummy", args[0])
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve value of the specified key '%s': %w", args[0], err)
	}

	// not found
	if value == nil || len(value) == 0 {
		return "", nil
	}

	return string(value), nil
}

func store(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("Incorrect arguments. Expecting a key and a value")
	}

	var asset SimpleAsset
	if err := json.Unmarshal([]byte(args[1]), &asset); err != nil {
		return "", fmt.Errorf("Unmarshal failed: %w", err)
	}

	asset.TxID = stub.GetTxID()

	b, err := json.Marshal(&asset)
	if err != nil {
		return "", fmt.Errorf("Marshal failed: %w", err)
	}

	if err := stub.PutState(args[0], b); err != nil {
		return "", fmt.Errorf("Failed to put key '%s' with value (%s) into the transaction's writeset: %w", args[0], args[1], err)
	}

	return "", nil
}

func storePrivateData(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	tmap, err := stub.GetTransient()
	if err != nil {
		return "", err
	}

	for key, value := range tmap {
		if err := stub.PutPrivateData("dummy", key, value); err != nil {
			return "", fmt.Errorf("Failed to put key '%s' with value (%s) into the transaction's private writeset: %w", key, value, err)
		}
	}

	return "", nil
}

func setEvent(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("Incorrect arguments. Expecting an event and its associated message")
	}

	if err := stub.SetEvent(args[0], []byte(args[1])); err != nil {
		return "", fmt.Errorf("Failed to set event %s: %w", args[0], err)
	}

	return "", nil
}
