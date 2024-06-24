package metrics

import (
	"errors"
	"sync"
)

type TxMapper struct {
	Data  map[string]*Tx
	mutex sync.Mutex
}

func NewTxMapper() ITxMapper {
	return &TxMapper{
		Data:  make(map[string]*Tx),
		mutex: sync.Mutex{},
	}
}

func (tm *TxMapper) AddEncryptedTx(identityPreimage []byte, encryptedTx []byte) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tx, exists := tm.Data[string(identityPreimage)]
	if !exists {
		tx = &Tx{}
		tm.Data[string(identityPreimage)] = tx
	}
	tx.EncryptedTx = encryptedTx
	return nil
}

func (tm *TxMapper) AddDecryptionData(identityPreimage []byte, dd *DecryptionData) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tx, exists := tm.Data[string(identityPreimage)]
	if !exists {
		tx = &Tx{}
		tm.Data[string(identityPreimage)] = tx
	}
	tx.DD = dd
	return nil
}

func (tm *TxMapper) AddBlockHash(slot uint64, blockHash []byte) error {
	for _, val := range tm.Data {
		if val.DD.Slot == slot {
			val.BlockHash = blockHash
			return nil
		}
	}
	return nil
}

func (tm *TxMapper) CanBeDecrypted(identityPreimage []byte) (bool, error) {
	tx, exists := tm.Data[string(identityPreimage)]
	if !exists {
		return false, nil
	}
	return len(tx.EncryptedTx) > 0 && tx.DD != nil, nil
}

func (tm *TxMapper) RemoveTx(identityPreimage []byte) error {
	if canBe, _ := tm.CanBeDecrypted(identityPreimage); !canBe {
		return errors.New("unable to remove Tx which cant be decrypted")
	}

	delete(tm.Data, string(identityPreimage))
	return nil
}
