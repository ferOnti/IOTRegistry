package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"runtime"
	"testing"

	proto "github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode/shim"

	IOTRegistryTX "github.com/Trusted-IoT-Alliance/IOTRegistry/IOTRegistryTX"
	"github.com/btcsuite/btcd/btcec"
)

/*
Notes from IOTRegistery tests

Private Key 1: 94d7fe7308a452fdf019a0424d9c48ba9b66bdbca565c6fa3b1bf9c646ebac20
 Public Key 1: 02ca4a8c7dc5090f924cde2264af240d76f6d58a5d2d15c8c5f59d95c70bd9e4dc

Private Key 2: 246d4fa59f0baa3329d3908659936ac2ac9c3539dc925977759cffe3c6316e19
 Public Key 2: 03442b817ad2154766a8f5192fc5a7506b7e52cdbf4fcf8e1bc33764698443c3c9

Private Key 3: 166cc93d9eadb573b329b5993b9671f1521679cea90fe52e398e66c1d6373abf
 Public Key 3: 02242a1c19bc831cd95a9e5492015043250cbc17d0eceb82612ce08736b8d753a6

Private Key 4: 01b756f231c72747e024ceee41703d9a7e3ab3e68d9b73d264a0196bd90acedf
 Public Key 4: 020f2b95263c4b3be740b7b3fda4c2f4113621c1a7a360713a2540eeb808519cd6

Unused:

Public Key: 02cb6d65b04c4b84502015f918fe549e95cad4f3b899359a170d4d7d438363c0ce
Private Key: 60977f22a920c9aa18d58d12cb5e90594152d7aa724bcce21484dfd0f4490b58
Hyperledger address hex 10734390011641497f489cb475743b8e50d429bb
Hyperledger address Base58: EHxhLN3Ft4p9jPkR31MJMEMee9G

Owner1 key
Public Key: 0278b76afbefb1e1185bc63ed1a17dd88634e0587491f03e9a8d2d25d9ab289ee7
Private Key: 7142c92e6eba38de08980eeb55b8c98bb19f8d417795adb56b6c4d25da6b26c5

Owner2 key
Public Key: 02e138b25db2e74c54f8ca1a5cf79e2d1ed6af5bd1904646e7dc08b6d7b0d12bfd
Private Key: b18b7d3082b3ff9438a7bf9f5f019f8a52fb64647ea879548b3ca7b551eefd65
*/

var hexChars = []rune("0123456789abcdef")
var alpha = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

/*
	testing tool for creating randomized string with a certain character makeup
*/
func randString(n int, kindOfString string) string {
	b := make([]rune, n)
	if kindOfString == "hex" {
		for i := range b {
			b[i] = hexChars[rand.Intn(len(hexChars))]
		}
	} else if kindOfString == "alpha" {
		for i := range b {
			b[i] = alpha[rand.Intn(len(alpha))]
		}
	} else {
		fmt.Println("randString() error: could not retrieve character list for random string generation")
		return ""
	}
	return string(b)
}

/*
	generates a signature for creating a registrant based on private key and message
*/
func createRegistrantSig(registrantName string, registrantPubkey string, data string, privateKeyStr string) (string, error) {
	privKeyByte, err := hex.DecodeString(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("error decoding hex encoded private key (%s)", privateKeyStr)
	}
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyByte)

	message := registrantName + ":" + registrantPubkey + ":" + data
	messageBytes := sha256.Sum256([]byte(message))
	sig, err := privKey.Sign(messageBytes[:])
	if err != nil {
		return "", fmt.Errorf("error signing message (%s) with private key (%s)", message, privateKeyStr)
	}
	return hex.EncodeToString(sig.Serialize()), nil
}

/*
	generates a signature for registering a thing based on private key and message
*/
func generateRegisterThingSig(registrantPubkey string, aliases []string, spec string, data string, privateKeyStr string) (string, error) {
	privKeyByte, err := hex.DecodeString(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("error decoding hex encoded private key (%s)", privateKeyStr)
	}
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyByte)
	message := registrantPubkey
	for _, identity := range aliases {
		message += ":" + identity
	}
	message += ":" + data
	message += ":" + spec
	messageBytes := sha256.Sum256([]byte(message))
	sig, err := privKey.Sign(messageBytes[:])
	if err != nil {
		return "", fmt.Errorf("error signing message (%s) with private key (%s)", message, privateKeyStr)
	}
	return hex.EncodeToString(sig.Serialize()), nil
}

/*
	generates a signature for registering a spec based on private key and message
*/
func generateRegisterSpecSig(specName string, registrantPubkey string, data string, privateKeyStr string) (string, error) {
	privKeyByte, err := hex.DecodeString(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("error decoding hex encoded private key (%s)", privateKeyStr)
	}
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyByte)

	message := specName + ":" + registrantPubkey + ":" + data
	messageBytes := sha256.Sum256([]byte(message))
	sig, err := privKey.Sign(messageBytes[:])
	if err != nil {
		return "", fmt.Errorf("error signing message (%s) with private key (%s)", message, privateKeyStr)
	}
	return hex.EncodeToString(sig.Serialize()), nil
}

func checkInit(t *testing.T, stub *shim.MockStub, args []string) {
	_, err := stub.MockInit("1", "", args)
	if err != nil {
		fmt.Println("INIT", args, "failed", err)
		t.FailNow()
	}
}

/*
	register a store type "Identites" to ledger by calling to Invoke()
*/
func createRegistrant(t *testing.T, stub *shim.MockStub, name string, data string,
	privateKeyString string, pubKeyString string) error {

	registrant := IOTRegistryTX.CreateRegistrantTX{}
	registrant.RegistrantName = name
	pubKeyBytes, err := hex.DecodeString(pubKeyString)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	registrant.RegistrantPubkey = pubKeyBytes
	registrant.Data = data

	//create signature
	hexOwnerSig, err := createRegistrantSig(registrant.RegistrantName, hex.EncodeToString(registrant.RegistrantPubkey), registrant.Data, privateKeyString)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	registrant.Signature, err = hex.DecodeString(hexOwnerSig)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	registrantBytes, err := proto.Marshal(&registrant)
	registrantBytesStr := hex.EncodeToString(registrantBytes)
	_, err = stub.MockInvoke("3", "createRegistrant", []string{registrantBytesStr})
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

/*
	registers a store type "Things" to ledger and an "Alias" store type for each member of string slice aliases by calling to Invoke()
*/
func registerThing(t *testing.T, stub *shim.MockStub, nonce []byte, aliases []string,
	registrantPubKey string, spec string, data string, privateKeyString string) error {

	thing := IOTRegistryTX.RegisterThingTX{}

	thing.Nonce = nonce
	thing.Aliases = aliases
	thing.RegistrantPubkey = registrantPubKey
	thing.Spec = spec

	//create signature
	hexThingSig, err := generateRegisterThingSig(registrantPubKey, aliases, spec, data, privateKeyString)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	thing.Signature, err = hex.DecodeString(hexThingSig)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	thing.Data = data
	thingBytes, err := proto.Marshal(&thing)
	thingBytesStr := hex.EncodeToString(thingBytes)
	_, err = stub.MockInvoke("3", "registerThing", []string{thingBytesStr})
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

/*
	registers a store type "Spec" to ledger by calling to Invoke()
*/
func registerSpec(t *testing.T, stub *shim.MockStub, specName string, registrantPubkey string,
	data string, privateKeyString string) error {

	registerSpec := IOTRegistryTX.RegisterSpecTX{}

	registerSpec.SpecName = specName
	registerSpec.RegistrantPubkey = registrantPubkey
	registerSpec.Data = data

	//create signature
	hexSpecSig, err := generateRegisterSpecSig(specName, registrantPubkey, data, privateKeyString)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	registerSpec.Signature, err = hex.DecodeString(hexSpecSig)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	registerSpecBytes, err := proto.Marshal(&registerSpec)
	registerSpecBytesStr := hex.EncodeToString(registerSpecBytes)
	_, err = stub.MockInvoke("3", "registerSpec", []string{registerSpecBytesStr})
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

/*
	tests whether two string slices are identical, returning true or false
*/
func testEq(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

/*
	Checks that different queries return expected values.
*/
func checkQuery(t *testing.T, stub *shim.MockStub, function string, index string, expected registryTest) error {
	var err error = nil
	var bytes []byte

	bytes, err = stub.MockQuery(function, []string{index})
	if err != nil {
		return fmt.Errorf("Query (%s):%s failed\n", function, err.Error())
	}
	if bytes == nil {
		return fmt.Errorf("Query (%s):%s failed to get value\n", function, err.Error())
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal(bytes, &jsonMap); err != nil {
		return fmt.Errorf("error unmarshalling json string %s", bytes)
	}
	fmt.Printf("JSON: %s\n", jsonMap)
	if function == "owner" {
		if jsonMap["RegistrantName"] != expected.RegistrantName {
			return fmt.Errorf("\nRegistrantName got       (%s)\nRegistrantName expected: (%s)\n", jsonMap["RegistrantName"], expected.RegistrantName)
		}
		if jsonMap["Pubkey"] != expected.pubKeyString {
			return fmt.Errorf("\nPubkey got       (%s)\nPubkey expected: (%s)\n", jsonMap["Pubkey"], expected.pubKeyString)
		}
	} else if function == "thing" {
		aliases := make([]string, len(jsonMap["Aliases"].([]interface{})))
		for i, element := range jsonMap["Aliases"].([]interface{}) {
			aliases[i] = element.(string)
		}
		if !(reflect.DeepEqual(aliases, expected.aliases)) {
			return fmt.Errorf("\nAlias got       (%x)\nAlias expected: (%x)\n", jsonMap["Aliases"], expected.aliases)
		}
		if jsonMap["RegistrantPubkey"] != expected.pubKeyString {
			return fmt.Errorf("\nRegistrantPubkey got       (%s)\nRegistrantPubkey expected: (%s)\n", jsonMap["RegistrantPubkey"], expected.pubKeyString)
		}
		if jsonMap["Data"] != expected.data {
			return fmt.Errorf("\nData got       (%s)\nData expected: (%s)\n", jsonMap["Data"], expected.data)
		}
		if jsonMap["SpecName"] != expected.specName {
			return fmt.Errorf("\nSpecName got       (%s)\nSpecName expected: (%s)\n", jsonMap["SpecName"], expected.specName)
		}
	} else if function == "spec" {
		if jsonMap["RegistrantPubkey"] != expected.pubKeyString {
			return fmt.Errorf("\nRegistrantPubkey got       (%s)\nRegistrantPubkey expected: (%s)\n", jsonMap["RegistrantPubkey"], expected.pubKeyString)
		}
		if jsonMap["Data"] != expected.data {
			return fmt.Errorf("\nData got       (%s)\nData expected: (%s)\n", jsonMap["Data"], expected.data)
		}
	}
	return nil
}

func HandleError(t *testing.T, err error) (b bool) {
	if err != nil {
		_, fn, line, _ := runtime.Caller(1)
		re := regexp.MustCompile("[^/]+$")
		t.Errorf("\x1b[32m\n[ERROR] in %s\tat line: %d\n%v\x1b[0m\n\n", re.FindAllString(fn, -1)[0], line, err)
		b = true
	}
	return
}

type registryTest struct {
	privateKeyString string
	pubKeyString     string
	RegistrantName   string
	data             string
	nonce            string
	specName         string
	aliases          []string
}

/*
	runs tests for four different hypothetical users: Alice, Gerald, Bob, and Cassandra
*/
func TestIOTRegistryChaincode(t *testing.T) {
	//declaring and initializing variables for all tests
	bst := new(IOTRegistry)
	stub := shim.NewMockStub("IOTRegistry", bst)

	var registryTestsSuccess = []registryTest{
		{ /*private key  1*/ "94d7fe7308a452fdf019a0424d9c48ba9b66bdbca565c6fa3b1bf9c646ebac20",
			/*public key 1*/ "02ca4a8c7dc5090f924cde2264af240d76f6d58a5d2d15c8c5f59d95c70bd9e4dc",
			"Alice", "test data" /*nonce:*/, "1f7b169c846f218ab552fa82fbf86758", "test spec", []string{"Foo", "Bar"}},

		{ /*private key  2*/ "246d4fa59f0baa3329d3908659936ac2ac9c3539dc925977759cffe3c6316e19",
			/*public key 2*/ "03442b817ad2154766a8f5192fc5a7506b7e52cdbf4fcf8e1bc33764698443c3c9",
			"Gerald", "test data 1" /*nonce:*/, "bf5c97d2d2a313e4f95957818a7b3edc", "test spec 2", []string{"one", "two", "three"}},

		{ /*private key  3*/ "166cc93d9eadb573b329b5993b9671f1521679cea90fe52e398e66c1d6373abf",
			/*public key 3*/ "02242a1c19bc831cd95a9e5492015043250cbc17d0eceb82612ce08736b8d753a6",
			"Bob", "test data 2" /*nonce:*/, "a492f2b8a67697c4f91d9b9332e82347", "test spec 3", []string{"ident4", "ident5", "ident6"}},

		{ /*private key  4*/ "01b756f231c72747e024ceee41703d9a7e3ab3e68d9b73d264a0196bd90acedf",
			/*public key 4*/ "020f2b95263c4b3be740b7b3fda4c2f4113621c1a7a360713a2540eeb808519cd6",
			"Cassandra", "test data 3" /*nonce:*/, "83de17bd7a25e0a9f6813976eadf26de", "test spec 4", []string{"ident7", "ident8", "ident9"}},
	}
	for _, test := range registryTestsSuccess {
		err := createRegistrant(t, stub, test.RegistrantName, test.data, test.privateKeyString, test.pubKeyString)
		if err != nil {
			HandleError(t, fmt.Errorf("%v\n", err))
			return
		}
		index := test.pubKeyString
		err = checkQuery(t, stub, "owner", index, test)
		if err != nil {
			HandleError(t, fmt.Errorf("%v\n", err))
		}
		nonceBytes, err := hex.DecodeString(test.nonce)
		if err != nil {
			HandleError(t, fmt.Errorf("Error decoding nonce bytes: %s", err.Error()))
		}
		err = registerThing(t, stub, nonceBytes, test.aliases, test.pubKeyString, test.specName, test.data, test.privateKeyString)
		if err != nil {
			HandleError(t, fmt.Errorf("%v\n", err))
		}
		for _, alias := range test.aliases {
			err = checkQuery(t, stub, "thing", alias, test)
			if err != nil {
				HandleError(t, fmt.Errorf("%v\n", err))
			}
		}

		err = registerSpec(t, stub, test.specName, test.pubKeyString, test.data, test.privateKeyString)
		if err != nil {
			HandleError(t, fmt.Errorf("%v\n", err))
		}
		index = test.specName
		err = checkQuery(t, stub, "spec", index, test)
		if err != nil {
			HandleError(t, fmt.Errorf("%v\n", err))
		}
	}
}
