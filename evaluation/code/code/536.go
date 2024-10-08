package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"math/rand"
	"reflect"
)

type Layer struct {
	Weights interface{} `json:"weights"`
	Biases  interface{} `json:"biases"`
}

type NeuralNetworkModel struct {
	Layers []Layer `json:"layers"`
}

type ClientData struct {
	ClientID string             `json:"clientID"`
	Data     NeuralNetworkModel `json:"data"`
	Round    int                `json:"round"`
	Epsilon  float64            `json:"epsilon"`
}

type ServerData struct {
	Data string `json:"data"`
	Round int `json:"round"`
}

type CurrentData struct {
	LatestRound int     `json:"round"`
	Tokens      float64 `json:"tokens"`
	RoundSeen   []int   `json:"roundSeen"`
}

type KeyRequests struct {
	Round    []int    `json:"round"`
	ClientID []string `json:"clientID"`
}

type SymmetricKey struct {
	Password string `json:"password"`
	IV       string `json:"iv"`
}

type SmartContract struct {
	contractapi.Contract
}

func addMatricesFloat64(a, b interface{}) (interface{}, error) {
	v1 := reflect.ValueOf(a)
	v2 := reflect.ValueOf(b)

	switch v1.Kind() {
	case reflect.Slice:
		res := reflect.MakeSlice(v1.Type(), v1.Len(), v1.Cap())
		for i := 0; i < v1.Len(); i++ {
			tempArray, err := addMatricesFloat64(v1.Index(i).Interface(), v2.Index(i).Interface())
			if err != nil {
				return nil, fmt.Errorf("failed to add matrices: %v", err)
			}
			res.Index(i).Set(reflect.ValueOf(tempArray))
		}
		return res.Interface(), nil
	case reflect.Float64:
		return v1.Float() + v2.Float(), nil
	default:
		panic("Unsupported type")
	}
}

func divideMatricesFloat64(a interface{}, b float64) (interface{}, error) {

	v1 := reflect.ValueOf(a)

	switch v1.Kind() {
	case reflect.Slice:
		res := reflect.MakeSlice(v1.Type(), v1.Len(), v1.Cap())
		for i := 0; i < v1.Len(); i++ {
			tempArray, err := divideMatricesFloat64(v1.Index(i).Interface(), b)
			if err != nil {
				return nil, fmt.Errorf("failed to divide matrices: %v", err)
			}
			res.Index(i).Set(reflect.ValueOf(tempArray))
		}
		return res.Interface(), nil
	case reflect.Float64:
		return v1.Float() / b, nil
	default:
		panic("Unsupported type")
	}
}

func getFirstDimensionLength(matrix interface{}) int {
	value := reflect.ValueOf(matrix)

	if value.Kind() != reflect.Slice {
		return 0
	}
	return value.Len()
}

func GetCNFromClientID(clientID string) (string, error) {

	decodedBytes, err := base64.StdEncoding.DecodeString(clientID)
	if err != nil {
		return "", fmt.Errorf("failed to decode client ID: %v", err)
	}

	clientID = string(decodedBytes)

	cn := ""
	for _, s := range clientID {
		if s == '=' {
			cn = ""
		} else if s == ',' {
			break
		} else {
			cn = cn + string(s)
		}
	}
	fmt.Printf("Client ID: %s, cn: %s\n", clientID, cn)
	return cn, nil
}

func getKeyFromCN(ctx contractapi.TransactionContextInterface, cn string, round int) (string, error) {

	prefix := "Client"
	if cn == "appserver" {
		prefix = "Server"
	}

	key, err := ctx.GetStub().CreateCompositeKey(prefix, []string{fmt.Sprint(round), cn})
	if err != nil {
		return "", fmt.Errorf("failed to create composite key: %v", err)
	}

	return key, nil
}

func (sc *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	clientID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return fmt.Errorf("failed to get client ID: %v", err)
	}

	cn, err := GetCNFromClientID(clientID)
	if err != nil {
		return fmt.Errorf("failed to get CN from client ID: %v", err)
	}

	key, err := getKeyFromCN(ctx, cn, 0)
	if err != nil {
		return fmt.Errorf("failed to get key from CN: %v", err)
	}

	InitClientData, err := ctx.GetStub().GetState(key)
	if err != nil {
		return fmt.Errorf("failed to read from world state: %v", err)
	}

	if InitClientData != nil {
		return fmt.Errorf("the client data %s already exists", cn)
	}

	clientData := ClientData{
		ClientID: clientID,
		Data: NeuralNetworkModel{
			Layers: []Layer{
				{
					Weights: nil,
					Biases:  nil,
				},
			},
		},
		Round:   0,
		Epsilon: 0,
	}

	clientDataAsBytes, err := json.Marshal(clientData)
	if err != nil {
		return fmt.Errorf("failed to marshal client data: %v", err)
	}

	err = ctx.GetStub().PutState(key, clientDataAsBytes)
	if err != nil {
		return fmt.Errorf("failed to write to world state: %v", err)
	}

	currentData := CurrentData{
		LatestRound: 0,
		Tokens:      1,
		RoundSeen:   []int{},
	}

	currentDataAsBytes, err := json.Marshal(currentData)
	if err != nil {
		return fmt.Errorf("failed to marshal latest round data: %v", err)
	}

	currentDataKey, err := ctx.GetStub().CreateCompositeKey("CurrentData", []string{cn})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}

	err = ctx.GetStub().PutState(currentDataKey, currentDataAsBytes)
	if err != nil {
		return fmt.Errorf("failed to write to world state: %v", err)
	}

	return nil
}

func getLatestRound(ctx contractapi.TransactionContextInterface, cn string) (int, error) {

	currentDataKey, err := ctx.GetStub().CreateCompositeKey("CurrentData", []string{cn})
	if err != nil {
		return 0, fmt.Errorf("failed to create composite key: %v", err)
	}

	currentDataAsBytes, err := ctx.GetStub().GetState(currentDataKey)
	if err != nil {
		return 0, fmt.Errorf("failed to read from world state: %v", err)
	}

	if currentDataAsBytes == nil {
		return 0, fmt.Errorf("the latest round data does not exist")
	}

	var currentData CurrentData
	err = json.Unmarshal(currentDataAsBytes, &currentData)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal latest round data: %v", err)
	}

	return currentData.LatestRound, nil
}

func (sc *SmartContract) PutData(ctx contractapi.TransactionContextInterface, data string, epsilon float64) error {
	clientID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return fmt.Errorf("failed to get client ID: %v", err)
	}

	cn, err := GetCNFromClientID(clientID)
	if err != nil {
		return fmt.Errorf("failed to get CN from client ID: %v", err)
	}

	latestRound, err := getLatestRound(ctx, cn)
	if err != nil {
		return fmt.Errorf("failed to get latest round: %v", err)
	}

	key, err := getKeyFromCN(ctx, cn, latestRound)
	if err != nil {
		return fmt.Errorf("failed to get key from CN: %v", err)
	}

	clientDataAsBytes, err := ctx.GetStub().GetState(key)
	if err != nil {
		return fmt.Errorf("failed to read from world state: %v", err)
	}

	var clientData ClientData

	err = json.Unmarshal(clientDataAsBytes, &clientData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal client data: %v", err)
	}

	serverCN := "appserver"
	serverRound, err := getLatestRound(ctx, serverCN)
	if err != nil {
		return fmt.Errorf("failed to get server round: %v", err)
	}

	roundNumber := latestRound + 1
	if serverRound > latestRound {
		roundNumber = serverRound + 1
	}

	var newModelData NeuralNetworkModel
	err = json.Unmarshal([]byte(data), &newModelData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal data: %v", err)
	}

	clientData.Data = newModelData
	clientData.Round = roundNumber
	clientData.Epsilon = epsilon

	clientDataAsBytes, err = json.Marshal(clientData)
	if err != nil {
		return fmt.Errorf("failed to marshal client data: %v", err)
	}

	key, err = getKeyFromCN(ctx, cn, roundNumber)
	if err != nil {
		return fmt.Errorf("failed to get key from CN: %v", err)
	}

	err = ctx.GetStub().PutState(key, clientDataAsBytes)
	if err != nil {
		return fmt.Errorf("failed to write to world state: %v", err)
	}

	currentDataKey, err := ctx.GetStub().CreateCompositeKey("CurrentData", []string{cn})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}

	currentDataAsBytes, err := ctx.GetStub().GetState(currentDataKey)
	if err != nil {
		return fmt.Errorf("failed to read from world state: %v", err)
	}

	var currentData CurrentData
	err = json.Unmarshal(currentDataAsBytes, &currentData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal latest round data: %v", err)
	}

	currentData.LatestRound = roundNumber

	currentDataAsBytes, err = json.Marshal(currentData)
	if err != nil {
		return fmt.Errorf("failed to marshal client data: %v", err)
	}

	err = ctx.GetStub().PutState(currentDataKey, currentDataAsBytes)
	if err != nil {
		return fmt.Errorf("failed to write to world state: %v", err)
	}

	return nil
}

// select random subset of size num from the list of clients
func SelectSubSet(ctx contractapi.TransactionContextInterface, num int, seed int, round int) ([]string, error) {

	rand.Seed(int64(seed))
	var clientList []string

	// get the list of all clients with round number round
	keysIter, err := ctx.GetStub().GetStateByPartialCompositeKey("Client", []string{fmt.Sprint(round)})
	if err != nil {
		return nil, fmt.Errorf("failed to get all keys: %v", err)
	}

	defer keysIter.Close()

	for keysIter.HasNext() {
		queryResponse, err := keysIter.Next()
		if err != nil {
			continue
		}
		clientList = append(clientList, queryResponse.Key)
	}

	// if less are there, send all
	if len(clientList) <= num {
		return clientList, nil
	}

	rand.Shuffle(len(clientList), func(i, j int) { clientList[i], clientList[j] = clientList[j], clientList[i] })

	return clientList[:num], nil
}

// get data of random num clients
func (sc *SmartContract) GetRoundData(ctx contractapi.TransactionContextInterface, num int, seed int) ([]NeuralNetworkModel, error) {

	var round int
	serverID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return []NeuralNetworkModel{}, fmt.Errorf("failed to get client ID: %v", err)
	}

	cn, err := GetCNFromClientID(serverID)
	if err != nil {
		return []NeuralNetworkModel{}, fmt.Errorf("failed to get CN from server ID: %v", err)
	}

	latestRound, err := getLatestRound(ctx, cn)
	if err != nil {
		return []NeuralNetworkModel{}, fmt.Errorf("failed to get latest round: %v", err)
	}

	round = latestRound + 1
	fmt.Printf("Round: %v\n", round)
	selectedClients, err := SelectSubSet(ctx, num, seed, round)

	fmt.Printf("Selected clients length: %v\n", len(selectedClients))
	if err != nil {
		return []NeuralNetworkModel{}, fmt.Errorf("failed to select subset: %v", err)
	}

	var clientModelData []NeuralNetworkModel

	for _, key := range selectedClients {

		clientDataAsBytes, err := ctx.GetStub().GetState(key)
		fmt.Printf("Currently working on client: %s\n", key)
		if err != nil {
			return []NeuralNetworkModel{}, fmt.Errorf("failed to read from world state: %v", err)
		}

		var clientData ClientData
		err = json.Unmarshal(clientDataAsBytes, &clientData)
		if err != nil {
			return []NeuralNetworkModel{}, fmt.Errorf("failed to unmarshal3 client data: %v", err)
		}

		if clientData.Round == round {
			clientCN, err := GetCNFromClientID(clientData.ClientID)
			if err != nil {
				return []NeuralNetworkModel{}, fmt.Errorf("failed to get CN from client ID: %v", err)
			}
			clientModelData = append(clientModelData, clientData.Data)
			currentDataKey, err := ctx.GetStub().CreateCompositeKey("CurrentData", []string{clientCN})
			if err != nil {
				return []NeuralNetworkModel{}, fmt.Errorf("failed to create composite key: %v", err)
			}

			currentDataAsBytes, err := ctx.GetStub().GetState(currentDataKey)
			if err != nil {
				return []NeuralNetworkModel{}, fmt.Errorf("failed to read from world state: %v", err)
			}

			var currentData CurrentData
			err = json.Unmarshal(currentDataAsBytes, &currentData)
			if err != nil {
				return []NeuralNetworkModel{}, fmt.Errorf("failed to unmarshal latest round data: %v", err)
			}
			currentData.Tokens = currentData.Tokens + (clientData.Epsilon+5)/20.0

			currentDataAsBytes, err = json.Marshal(currentData)
			if err != nil {
				return []NeuralNetworkModel{}, fmt.Errorf("failed to marshal client data: %v", err)
			}
			err = ctx.GetStub().PutState(currentDataKey, currentDataAsBytes)
			if err != nil {
				fmt.Printf("failed to write to world state for client %s, %v\n", clientCN, err)
				continue
			}
			fmt.Printf("Client %s has %v tokens\n", clientCN, currentData.Tokens)
		}

	}

	if len(clientModelData) == 0 {
		return []NeuralNetworkModel{}, fmt.Errorf("no data for round %v", round)
	}

	fmt.Printf("Client model data length: %v\n", len(clientModelData))

	return clientModelData, nil
}

func (sc *SmartContract) PutGlobalWeights(ctx contractapi.TransactionContextInterface, data string, round int) error {

	serverData := ServerData{
		Data: data,
		Round: round,
	}

	serverDataAsBytes, err := json.Marshal(serverData)
	if err != nil {
		return fmt.Errorf("failed to marshal server data: %v", err)
	}

	serverID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return fmt.Errorf("failed to get client ID: %v", err)
	}

	cn, err := GetCNFromClientID(serverID)
	if err != nil {
		return fmt.Errorf("failed to get CN from client ID: %v", err)
	}

	serverKey, err := getKeyFromCN(ctx, cn, round)
	if err != nil {
		return fmt.Errorf("failed to get key from CN: %v", err)
	}

	err = ctx.GetStub().PutState(serverKey, serverDataAsBytes)
	if err != nil {
		return fmt.Errorf("failed to write to world state: %v", err)
	}

	return nil
}

func (sc *SmartContract) RequestKey(ctx contractapi.TransactionContextInterface, round int) error {
	clientID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return fmt.Errorf("failed to get client ID: %v", err)
	}

	cn, err := GetCNFromClientID(clientID)
	if err != nil {
		return fmt.Errorf("failed to get CN from client ID: %v", err)
	}

	keyRequests, err := ctx.GetStub().GetState("KeyRequests")
	if err != nil {
		return fmt.Errorf("failed to read from world state: %v", err)
	}

	var keyRequestsData KeyRequests
	err = json.Unmarshal(keyRequests, &keyRequestsData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal key requests: %v", err)
	}

	keyRequestsData.Round = append(keyRequestsData.Round, round)
	keyRequestsData.ClientID = append(keyRequestsData.ClientID, cn)

	keyRequests, err = json.Marshal(keyRequestsData)
	if err != nil {
		return fmt.Errorf("failed to marshal key requests: %v", err)
	}

	err = ctx.GetStub().PutState("KeyRequests", keyRequests)
	if err != nil {
		return fmt.Errorf("failed to write to world state: %v", err)
	}

	return nil
}

func (sc *SmartContract) GetKeyRequests(ctx contractapi.TransactionContextInterface) (KeyRequests, error) {
	keyRequests, err := ctx.GetStub().GetState("KeyRequests")
	if err != nil {
		return KeyRequests{}, fmt.Errorf("failed to read from world state: %v", err)
	}

	var keyRequestsData KeyRequests
	err = json.Unmarshal(keyRequests, &keyRequestsData)
	if err != nil {
		return KeyRequests{}, fmt.Errorf("failed to unmarshal key requests: %v", err)
	}

	var finalKeyRequests KeyRequests = keyRequestsData

	// clear the key requests
	keyRequestsData.Round = []int{}
	keyRequestsData.ClientID = []string{}

	keyRequests, err = json.Marshal(keyRequestsData)
	if err != nil {
		return KeyRequests{}, fmt.Errorf("failed to marshal key requests: %v", err)
	}

	err = ctx.GetStub().PutState("KeyRequests", keyRequests)
	if err != nil {
		return KeyRequests{}, fmt.Errorf("failed to write to world state: %v", err)
	}

	return finalKeyRequests, nil
}

func (sc *SmartContract) PutSymmetricKey(ctx contractapi.TransactionContextInterface, clientID string, round int, privateDataCollection string) error {

	// get transient data
	transMap, err := ctx.GetStub().GetTransient()

	if err != nil {
		return fmt.Errorf("failed to get transient field: %v", err)
	}

	password := transMap["password"]
	iv := transMap["iv"]
	
	var symmetricKey SymmetricKey
	symmetricKey.Password = string(password)
	symmetricKey.IV = string(iv)

	symmetricKeyAsBytes, err := json.Marshal(symmetricKey)
	if err != nil {
		return fmt.Errorf("failed to marshal symmetric key: %v", err)
	}
	
	// remove token from client
	cn, err := GetCNFromClientID(clientID)
	if err != nil {
		return fmt.Errorf("failed to get CN from client ID: %v", err)
	}
	
	currentDataKey, err := ctx.GetStub().CreateCompositeKey("CurrentData", []string{cn})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}
	
	currentDataAsBytes, err := ctx.GetStub().GetState(currentDataKey)
	if err != nil {
		return fmt.Errorf("failed to read from world state: %v", err)
	}
	
	var currentData CurrentData
	err = json.Unmarshal(currentDataAsBytes, &currentData)
	
	if err != nil {
		return fmt.Errorf("failed to unmarshal latest round data: %v", err)
	}
	
	if currentData.Tokens < 1 {
		return fmt.Errorf("client %s has no tokens", cn)
	}
	currentData.Tokens = currentData.Tokens - 1

	key := fmt.Sprint(round)
	if err != nil {
		return fmt.Errorf("failed to get key from CN: %v", err)
	}
	
	err = ctx.GetStub().PutPrivateData(privateDataCollection, key, symmetricKeyAsBytes)

	if err != nil {
		return fmt.Errorf("failed to write to private data collection: %v", err)
	}
	
	currentDataAsBytes, err = json.Marshal(currentData)

	if err != nil {
		return fmt.Errorf("failed to marshal client data: %v", err)
	}

	err = ctx.GetStub().PutState(currentDataKey, currentDataAsBytes)

	if err != nil {
		return fmt.Errorf("failed to write to world state: %v", err)
	}

	return nil
}

func (sc *SmartContract) GetSymmetricKey(ctx contractapi.TransactionContextInterface, round int, privateDataCollection string) (SymmetricKey, error) {

	key := fmt.Sprint(round)
	symmetricKeyAsBytes, err := ctx.GetStub().GetPrivateData(privateDataCollection, key)

	if err != nil {
		return SymmetricKey{}, fmt.Errorf("failed to read from private data collection: %v", err)
	}

	var symmetricKey SymmetricKey
	err = json.Unmarshal(symmetricKeyAsBytes, &symmetricKey)

	if err != nil {
		return SymmetricKey{}, fmt.Errorf("failed to unmarshal symmetric key: %v", err)
	}

	return symmetricKey, nil
}

func getData(ctx contractapi.TransactionContextInterface, cn string, round int) (ClientData, error) {

	key, err := getKeyFromCN(ctx, cn, round)
	if err != nil {
		return ClientData{}, fmt.Errorf("failed to get key from CN: %v", err)
	}

	clientDataAsBytes, err := ctx.GetStub().GetState(key)
	if err != nil {
		return ClientData{}, fmt.Errorf("failed to read from world state: %v", err)
	}

	if clientDataAsBytes == nil {
		return ClientData{}, fmt.Errorf("the client data %s does not exist", cn)
	}
	var clientData ClientData
	err = json.Unmarshal(clientDataAsBytes, &clientData)
	if err != nil {
		return ClientData{}, fmt.Errorf("failed to unmarshal client data: %v", err)
	}

	return clientData, nil
}

// get data of requested client
func (sc *SmartContract) GetData(ctx contractapi.TransactionContextInterface) (ClientData, error) {
	clientID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return ClientData{}, fmt.Errorf("failed to get client ID: %v", err)
	}

	cn, err := GetCNFromClientID(clientID)
	if err != nil {
		return ClientData{}, fmt.Errorf("failed to get CN from client ID: %v", err)
	}

	latestRound, err := getLatestRound(ctx, cn)
	if err != nil {
		return ClientData{}, fmt.Errorf("failed to get latest round: %v", err)
	}

	return getData(ctx, cn, latestRound)
}

// get the data pushed by the server
func (sc *SmartContract) GetResult(ctx contractapi.TransactionContextInterface, round int) (NeuralNetworkModel, error) {

	clientID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return NeuralNetworkModel{}, fmt.Errorf("failed to get client ID: %v", err)
	}

	clientCN, err := GetCNFromClientID(clientID)
	if err != nil {
		return NeuralNetworkModel{}, fmt.Errorf("failed to get CN from client ID: %v", err)
	}

	serverCN := "appserver"

	serverData, err := getData(ctx, serverCN, round)
	if err != nil {
		return NeuralNetworkModel{}, fmt.Errorf("failed to get server data: %v for ", err)
	}

	currentDataKey, err := ctx.GetStub().CreateCompositeKey("CurrentData", []string{clientCN})
	if err != nil {
		return NeuralNetworkModel{}, fmt.Errorf("failed to create composite key: %v", err)
	}

	currentDataAsBytes, err := ctx.GetStub().GetState(currentDataKey)
	if err != nil {
		return NeuralNetworkModel{}, fmt.Errorf("failed to read from world state: %v", err)
	}

	var currentData CurrentData
	err = json.Unmarshal(currentDataAsBytes, &currentData)
	if err != nil {
		return NeuralNetworkModel{}, fmt.Errorf("failed to unmarshal latest round data: %v", err)
	}

	// check if client has already seen the round
	for _, roundSeen := range currentData.RoundSeen {
		if roundSeen == round {
			fmt.Printf("Client %s has already seen round %v\n", clientCN, round)
			return serverData.Data, nil
		}
	}

	if currentData.Tokens < 1 {
		fmt.Printf("Client %s has no tokens\n", clientCN)
		return NeuralNetworkModel{Layers: []Layer{{Weights: nil, Biases: nil}}}, nil
	}

	currentData.RoundSeen = append(currentData.RoundSeen, round)
	currentData.Tokens = currentData.Tokens - 1 //consuming tokens from client

	fmt.Printf("Client %s got data for round %v\n", clientCN, currentData)
	currentDataAsBytes, err = json.Marshal(currentData)
	if err != nil {
		return NeuralNetworkModel{}, fmt.Errorf("failed to marshal client data: %v", err)
	}

	err = ctx.GetStub().PutState(currentDataKey, currentDataAsBytes)
	if err != nil {
		return NeuralNetworkModel{}, fmt.Errorf("failed to write to world state: %v", err)
	}

	return serverData.Data, nil
}

func main() {

	chaincode, err := contractapi.NewChaincode(new(SmartContract))

	if err != nil {
		fmt.Printf("Error create testchaincode chaincode: %s", err.Error())
		return
	}

	err = chaincode.Start()

	if err != nil {
		fmt.Printf("Error starting testchaincode chaincode: %s", err.Error())
	}
}
