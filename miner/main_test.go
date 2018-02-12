package miner

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/mchetelat/bazo_miner/p2p"
	"github.com/mchetelat/bazo_miner/protocol"
	"github.com/mchetelat/bazo_miner/storage"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"testing"
	"fmt"
)

//Some user accounts for testing
const (
	PubA1 = "c2be9abbeaec39a066c2a09cee23bb9ab2a0b88f2880b1e785b4d317adf0dc7c"
	PubA2 = "8ce020fde838d9c443f6c93345dafe7fd74f091c4d2f30b37e2453679a257ed5"
	PrivA = "ba127fa8f802b008b9cdb58f4e44809d48f1b000cff750dda9cd6b312395c1c5"
	PubB1 = "5d7eefd58e3d2f309471928ab4bbd104e52973372c159fa652b8ca6b57ff68b8"
	PubB2 = "ab301a6a77b201c416ddc13a2d33fdf200a5302f6f687e0ea09085debaf8a1d9"
	PrivB = "7a0a9babcc97ea7991ed67ed7f800f70c5e04e99718960ad8efab2ca052f00c7"
)

//Root account for testing
const (
	RootPub1 = "f894ba7a24c1c324bc4b0a833d4b076a0e0f675a380fb7e782672c6568aaab06"
	RootPub2 = "69ddbc62f79cb521411840d83ff0abf941a8e717d81af3dfc2973f1bac30308a"
	RootPriv = "1c90d27e539d035512d27d072f7b514753157fa1591ff5c5a8a9ef642449d291"
)

//Globally accessible values for all other tests, (root)account-related
var (
	accA, accB, minerAcc             *protocol.Account
	PrivKeyA, PrivKeyB, MinerPrivKey ecdsa.PrivateKey
	PubKeyA, PubKeyB                 ecdsa.PublicKey
	RootPrivKey                      ecdsa.PrivateKey
	GenesisBlock					 *protocol.Block
)

//Create some accounts that are used by the tests
func addTestingAccounts() {

	accA, accB, minerAcc = new(protocol.Account), new(protocol.Account), new(protocol.Account)

	puba1, _ := new(big.Int).SetString(PubA1, 16)
	puba2, _ := new(big.Int).SetString(PubA2, 16)
	priva, _ := new(big.Int).SetString(PrivA, 16)
	PubKeyA = ecdsa.PublicKey{
		elliptic.P256(),
		puba1,
		puba2,
	}
	PrivKeyA = ecdsa.PrivateKey{
		PubKeyA,
		priva,
	}

	pubb1, _ := new(big.Int).SetString(PubB1, 16)
	pubb2, _ := new(big.Int).SetString(PubB2, 16)
	privb, _ := new(big.Int).SetString(PrivB, 16)
	PubKeyB = ecdsa.PublicKey{
		elliptic.P256(),
		pubb1,
		pubb2,
	}
	PrivKeyB = ecdsa.PrivateKey{
		PubKeyB,
		privb,
	}

	copy(accA.Address[0:32], PrivKeyA.PublicKey.X.Bytes())
	copy(accA.Address[32:64], PrivKeyA.PublicKey.Y.Bytes())
	hashA := protocol.SerializeHashContent(accA.Address)

	//This one is just for testing purposes
	copy(accB.Address[0:32], PrivKeyB.PublicKey.X.Bytes())
	copy(accB.Address[32:64], PrivKeyB.PublicKey.Y.Bytes())
	hashB := protocol.SerializeHashContent(accB.Address)

	//just to bootstrap
	storage.State[hashA] = accA
	storage.State[hashB] = accB

	minerPrivKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	var pubKey [64]byte
	var shortMiner [8]byte
	copy(pubKey[:32], minerPrivKey.X.Bytes())
	copy(pubKey[32:], minerPrivKey.Y.Bytes())
	minerHash := protocol.SerializeHashContent(pubKey)
	copy(shortMiner[:], minerHash[0:8])
	minerAcc.Address = pubKey
	storage.State[minerHash] = minerAcc
}

//Create some root accounts that are used by the tests
func addRootAccounts() {

	var pubKey [64]byte

	pub1, _ := new(big.Int).SetString(RootPub1, 16)
	pub2, _ := new(big.Int).SetString(RootPub2, 16)
	priv, _ := new(big.Int).SetString(RootPriv, 16)
	PubKeyA = ecdsa.PublicKey{
		elliptic.P256(),
		pub1,
		pub2,
	}
	RootPrivKey = ecdsa.PrivateKey{
		PubKeyA,
		priv,
	}

	copy(pubKey[32-len(pub1.Bytes()):32], pub1.Bytes())
	copy(pubKey[64-len(pub2.Bytes()):], pub2.Bytes())

	rootHash := protocol.SerializeHashContent(pubKey)

	rootAcc := protocol.Account{Address: pubKey}

	//create root file
	file, _ := os.Create(storage.DEFAULT_KEY_FILE_NAME)
	_, _ = file.WriteString(RootPub1 + "\n")
	_, _ = file.WriteString(RootPub2 + "\n")
	_, _ = file.WriteString(RootPriv + "\n")

	var hashedSeed [32]byte

	validatorAccount = storage.DEFAULT_KEY_FILE_NAME

	//create and store an initial seed for the root account
	seed := protocol.CreateRandomSeed()
	hashedSeed = protocol.SerializeHashContent(seed)
	_ = storage.AppendNewSeed(storage.SEED_FILE_NAME, storage.SeedJson{fmt.Sprintf("%x",string(hashedSeed[:])), string(seed[:])})

	rootAcc.HashedSeed = hashedSeed

	//set funds of root account in order to avoid zero division for PoS
	rootAcc.Balance = 10000000000
	rootAcc.IsStaking = true

	storage.State[rootHash] = &rootAcc
	storage.RootKeys[rootHash] = &rootAcc
}

//The state changes (accounts, funds, system parameters etc.) need to be reverted before any new test starts
//So every test has the same view on the blockchain
func cleanAndPrepare() {

	storage.DeleteAll()

	tmpState := make(map[[32]byte]*protocol.Account)
	tmpRootKeys := make(map[[32]byte]*protocol.Account)

	storage.State = tmpState
	storage.RootKeys = tmpRootKeys

	lastBlock = nil

	globalBlockCount = -1
	localBlockCount = -1

	//Prepare system parameters
	targetTimes = []timerange{}
	currentTargetTime = new(timerange)
	target = append(target, 8)

	var tmpSlice []Parameters
	tmpSlice = append(tmpSlice, NewDefaultParameters())

	slashingDict = make(map[[32]byte]SlashingProof)

	parameterSlice = tmpSlice
	activeParameters = &tmpSlice[0]

	GenesisBlock = newBlock([32]byte{}, [32]byte{}, [32]byte{}, 0)

	var genesisSeedSlice [32]byte
	copy(genesisSeedSlice[:], storage.GENESIS_SEED)
	GenesisBlock.Seed = genesisSeedSlice

	collectStatistics(GenesisBlock)
	storage.WriteClosedBlock(GenesisBlock)
	storage.WriteLastClosedBlock(GenesisBlock)

	addTestingAccounts()
	addRootAccounts()

	//Some meaningful balance to simplify testing
	minerAcc.Balance = 0
	accA.Balance = 123232345678
	accB.Balance = 823237654321
	accA.TxCnt = 0
	accB.TxCnt = 0
}

func TestMain(m *testing.M) {
	storage.Init("127.0.0.1:8000", "test.db")
	p2p.Init("127.0.0.1:8000")

	addTestingAccounts()
	addRootAccounts()
	//We don't want logging msgs when testing, we have designated messages
	logger = log.New(nil, "", 0)
	logger.SetOutput(ioutil.Discard)
	os.Exit(m.Run())

	storage.TearDown()
}