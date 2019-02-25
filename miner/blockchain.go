package miner

import (
	"github.com/bazo-blockchain/bazo-miner/crypto"
	"github.com/bazo-blockchain/bazo-miner/protocol"
	"github.com/bazo-blockchain/bazo-miner/storage"
	"golang.org/x/crypto/ed25519"
	"log"
	"os"
	"sync"
)

var (
	logger                       *log.Logger
	blockValidation              = &sync.Mutex{}
	parameterSlice               []Parameters
	activeParameters             *Parameters
	uptodate                     bool
	slashingDict                 = make(map[[32]byte]SlashingProof)
	validatorAccAddress          [32]byte
	multisigPubKey               ed25519.PublicKey
	commPrivKey, rootCommPrivKey ed25519.PrivateKey
	blockchainSize               = 0
	FileConnectionsLog         *os.File
	FileConnections   	       *os.File


)

//Miner entry point
func Init(validatorWallet, multisigWallet ed25519.PublicKey , rootWallet, validatorCommitment, rootCommitment ed25519.PrivateKey) {
	var err error


	validatorAccAddress = crypto.GetAddressFromPubKeyED(validatorWallet)
	multisigPubKey = multisigWallet
	commPrivKey = validatorCommitment
	rootCommPrivKey = rootCommitment

	//Set up logger.
	logger = storage.InitLogger()
	logger.Printf("\n\n\n-------------------- START MINER ---------------------")

	parameterSlice = append(parameterSlice, NewDefaultParameters())
	activeParameters = &parameterSlice[0]

	//Initialize root key.
	initRootKey(ed25519.PublicKey(rootWallet[32:]))
	if err != nil {
		logger.Printf("Could not create a root account.\n")
	}

	currentTargetTime = new(timerange)
	target = append(target, 15)

	initialBlock, err := initState()
	if err != nil {
		logger.Printf("Could not set up initial state: %v.\n", err)
		return
	}

	logger.Printf("ActiveConfigParams: \n%v\n------------------------------------------------------------------------\n\nBAZO is Running\n\n", activeParameters)

	//this is used to generate the state with aggregated transactions.
	for _, tx := range storage.ReadAllBootstrapReceivedTransactions() {
		storage.DeleteOpenTx(tx)
		storage.WriteClosedTx(tx)
	}
	storage.DeleteBootstrapReceivedMempool()

	//Start to listen to network inputs (txs and blocks).
	go incomingData()
	mining(initialBlock)
}

//Mining is a constant process, trying to come up with a successful PoW.
func mining(initialBlock *protocol.Block) {
	currentBlock := newBlock(initialBlock.Hash, initialBlock.HashWithoutTx, [crypto.COMM_PROOF_LENGTH_ED]byte{}, initialBlock.Height+1)

	for {
		err := finalizeBlock(currentBlock)
		if err != nil {
			logger.Printf("%v\n", err)
		} else {
			logger.Printf("Block mined (%x)\n", currentBlock.Hash[0:8])
		}

		if err == nil {
			err := validate(currentBlock, false)
			if err == nil {
				//Only broadcast the block if it is valid.
				broadcastBlock(currentBlock)
				logger.Printf("Validated block (mined): %vState:\n%v", currentBlock, getState())

			} else {
				logger.Printf("Mined block (%x) could not be validated: %v\n", currentBlock.Hash[0:8], err)
			}
		}

		storage.ReadMempool()

		//This is the same mutex that is claimed at the beginning of a block validation. The reason we do this is
		//that before start mining a new block we empty the mempool which contains tx data that is likely to be
		//validated with block validation, so we wait in order to not work on tx data that is already validated
		//when we finish the block.
		blockValidation.Lock()
		nextBlock := newBlock(lastBlock.Hash, lastBlock.HashWithoutTx, [crypto.COMM_PROOF_LENGTH_ED]byte{}, lastBlock.Height+1)
		currentBlock = nextBlock
		prepareBlock(currentBlock)
		blockValidation.Unlock()
	}
}

//At least one root key needs to be set which is allowed to create new accounts.
//At least one root key needs to be set which is allowed to create new accounts.
func initRootKey(rootKey ed25519.PublicKey) error {
	address := crypto.GetAddressFromPubKeyED(rootKey)
	addressHash := protocol.SerializeHashContent(address)

	var commPubKey [crypto.COMM_KEY_LENGTH_ED]byte
	copy(commPubKey[:], rootCommPrivKey[32:])

	rootAcc := protocol.NewAccount(address, [32]byte{}, activeParameters.Staking_minimum, true, commPubKey, nil, nil)
	storage.State[addressHash] = &rootAcc
	storage.RootKeys[addressHash] = &rootAcc

	return nil
}

//func CalculateBlockchainSize(currentBlockSize int) {
//	blockchainSize = blockchainSize + currentBlockSize
//	logger.Printf("Blockchain size is: %v bytes\n", blockchainSize)
//}
