package miner

import (
	"errors"
	"github.com/bazo-blockchain/bazo-miner/protocol"
	"github.com/bazo-blockchain/bazo-miner/storage"
)

//Already validated block but not part of the current longest chain.
//No need for an additional state mutex, because this function is called while the blockValidation mutex is actively held.
func rollback(b *protocol.Block) error {
	accTxSlice, fundsTxSlice, configTxSlice, stakeTxSlice, aggTxSlice, iotTxSlice, err := preValidateRollback(b)
	if err != nil {
		return err
	}

	data := blockData{accTxSlice, fundsTxSlice, configTxSlice, stakeTxSlice, aggTxSlice, iotTxSlice, b}

	//Going back to pre-block system parameters before the state is rolled back.
	configStateChangeRollback(data.configTxSlice, b.Hash)

	//TODO Does not throw error but crashes
	validateStateRollback(data)

	postValidateRollback(data)

	return nil
}

func preValidateRollback(b *protocol.Block) (accTxSlice []*protocol.AccTx, fundsTxSlice []*protocol.FundsTx,
	configTxSlice []*protocol.ConfigTx, stakeTxSlice []*protocol.StakeTx, aggTxSlice []*protocol.AggTx,iotTxSlice []*protocol.IotTx, err error) {
	//Fetch all transactions from closed storage.
	for _, hash := range b.AccTxData {
		var accTx *protocol.AccTx
		tx := storage.ReadClosedTx(hash)
		if tx == nil {
			//This should never happen, because all validated transactions are in closed storage.
			return nil, nil, nil, nil, nil, nil, errors.New("CRITICAL: Validated accTx was not in the confirmed tx storage")
		} else {
			accTx = tx.(*protocol.AccTx)
		}
		accTxSlice = append(accTxSlice, accTx)
	}

	for _, hash := range b.FundsTxData {
		var fundsTx *protocol.FundsTx
		tx := storage.ReadClosedTx(hash)
		if tx == nil {
			return nil, nil, nil, nil,nil, nil,errors.New("CRITICAL: Validated fundsTx was not in the confirmed tx storage")
		} else {
			fundsTx = tx.(*protocol.FundsTx)
		}
		fundsTxSlice = append(fundsTxSlice, fundsTx)
	}

	for _, hash := range b.ConfigTxData {
		var configTx *protocol.ConfigTx
		tx := storage.ReadClosedTx(hash)
		if tx == nil {
			return nil, nil, nil, nil, nil,nil, errors.New("CRITICAL: Validated configTx was not in the confirmed tx storage")
		} else {
			configTx = tx.(*protocol.ConfigTx)
		}
		configTxSlice = append(configTxSlice, configTx)
	}

	for _, hash := range b.StakeTxData {
		var stakeTx *protocol.StakeTx
		tx := storage.ReadClosedTx(hash)
		if tx == nil {
			return nil, nil, nil, nil, nil,nil, errors.New("CRITICAL: Validated stakeTx was not in the confirmed tx storage")
		} else {
			stakeTx = tx.(*protocol.StakeTx)
		}
		stakeTxSlice = append(stakeTxSlice, stakeTx)
	}

	for _, hash := range b.IoTTxData {
		var IoTTx *protocol.IotTx
		tx := storage.ReadClosedTx(hash)
		if tx == nil {
			return nil, nil, nil, nil, nil, nil,errors.New("CRITICAL: Aggregated Transaction was not in the confirmed tx storage")
		} else {
			IoTTx = tx.(*protocol.IotTx)
		}
		iotTxSlice = append(iotTxSlice, IoTTx)
	}

	for _, hash := range b.AggTxData {
		var aggTx *protocol.AggTx
		tx := storage.ReadClosedTx(hash)
		if tx == nil {
			return nil, nil, nil, nil, nil, nil,errors.New("CRITICAL: Aggregated Transaction was not in the confirmed tx storage")
		} else {
			aggTx = tx.(*protocol.AggTx)
		}
		aggTxSlice = append(aggTxSlice, aggTx)
	}

	return accTxSlice, fundsTxSlice, configTxSlice, stakeTxSlice, aggTxSlice, iotTxSlice,nil
}

func validateStateRollback(data blockData) {
	collectSlashRewardRollback(activeParameters.Slash_reward, data.block)
	collectBlockRewardRollback(activeParameters.Block_reward, data.block.Beneficiary)
	collectTxFeesRollback(data.accTxSlice, data.fundsTxSlice, data.configTxSlice, data.stakeTxSlice, data.block.Beneficiary)
	stakeStateChangeRollback(data.stakeTxSlice)
	fundsStateChangeRollback(data.fundsTxSlice)
	aggregatedSenderStateRollback(data.aggTxSlice)
	accStateChangeRollback(data.accTxSlice)
}

func postValidateRollback(data blockData) {
	//Put all validated txs into invalidated state.
	for _, tx := range data.accTxSlice {
		storage.WriteOpenTx(tx)
		storage.DeleteClosedTx(tx)
	}

	for _, tx := range data.fundsTxSlice {
		storage.WriteOpenTx(tx)
		storage.DeleteClosedTx(tx)
	}

	for _, tx := range data.configTxSlice {
		storage.WriteOpenTx(tx)
		storage.DeleteClosedTx(tx)
	}

	for _, tx := range data.stakeTxSlice {
		storage.WriteOpenTx(tx)
		storage.DeleteClosedTx(tx)
	}

	for _, tx := range data.aggTxSlice {

		//Reopen FundsTx per aggTx
		for _, aggregatedTxHash := range tx.AggregatedTxSlice {
			trx := storage.ReadClosedTx(aggregatedTxHash)
			storage.WriteOpenTx(trx)
			storage.DeleteClosedTx(trx)
		}

		//Delete AggTx. No need to write in OpenTx, because it will be created newly.
		logger.Printf("Rolled Back AggTx: %x, %v", tx.Hash(), tx.Hash())
		storage.DeleteClosedTx(tx)
	}

	//CalculateBlockchainSize(-int(data.block.GetSize()))

	collectStatisticsRollback(data.block)

	//For transactions we switch from closed to open. However, we do not write back blocks
	//to open storage, because in case of rollback the chain they belonged to is likely to starve.
	storage.DeleteClosedBlock(data.block.Hash)
	storage.WriteToReceivedStash(data.block) //Write it to received stash, it will be deleted after X new blocks.

	//Save the previous block as the last closed block.
	storage.DeleteAllLastClosedBlock()
	storage.WriteLastClosedBlock(storage.ReadClosedBlock(data.block.PrevHash))
}
