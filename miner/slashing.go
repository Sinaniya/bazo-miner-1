package miner

import (
	"errors"
	"github.com/bazo-blockchain/bazo-miner/protocol"
	"github.com/bazo-blockchain/bazo-miner/storage"
)

type SlashingProof struct {
	ConflictingBlockHash1 [32]byte
	ConflictingBlockHash2 [32]byte
	ConflictingBlockHashWithoutTx1 [32]byte
	ConflictingBlockHashWithoutTx2 [32]byte
}

//Find a proof where a validator votes on two different chains within the slashing window
func seekSlashingProof(block *protocol.Block) error {
	//check if block is being added to your chain
	lastClosedBlock := storage.ReadLastClosedBlock()
	if lastClosedBlock == nil {
		return errors.New("Latest block not found.")
	}

	//When the block is added ontop of your chain then there is no slashing needed
	if lastClosedBlock.Hash == block.Hash || lastClosedBlock.Hash == block.PrevHash {
		return nil
	} else {
		//Get the latest blocks and check if there is proof for multi-voting within the slashing window
		prevBlocks := storage.ReadAllClosedBlocks()

		if prevBlocks == nil {
			return nil
		}
		for _, prevBlock := range prevBlocks {
			if IsInSameChain(prevBlock, block) {
				return nil
			}
			if prevBlock.Beneficiary == block.Beneficiary &&
				(uint64(prevBlock.Height) < uint64(block.Height)+activeParameters.Slashing_window_size ||
					uint64(block.Height) < uint64(prevBlock.Height)+activeParameters.Slashing_window_size) {
				slashingDict[block.Beneficiary] = SlashingProof{ConflictingBlockHash1: block.Hash, ConflictingBlockHash2: prevBlock.Hash, ConflictingBlockHashWithoutTx1: block.HashWithoutTx, ConflictingBlockHashWithoutTx2: block.PrevHashWithoutTx}
			}
		}
	}
	return nil
}

//Check if two blocks are part of the same chain or if they appear in two competing chains
func IsInSameChain(b1, b2 *protocol.Block) bool {
	var higherBlock, lowerBlock  *protocol.Block

	if b1.Height == b2.Height {
		return false
	}

	if b1.Height > b2.Height {
		higherBlock = b1
		lowerBlock = b2
	} else {
		higherBlock = b2
		lowerBlock = b1
	}

	for higherBlock.Height > 0 {
		higherBlock = storage.ReadClosedBlock(higherBlock.PrevHash)
		//Check blocks without transactions
		if higherBlock == nil {
			higherBlock = storage.ReadClosedBlockWithoutTx(higherBlock.PrevHashWithoutTx)
		}
		if higherBlock.Hash == lowerBlock.Hash {
			return true
		}
	}

	return false
}
