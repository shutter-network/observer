package metrics

import (
	"encoding/hex"
	"errors"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type TxMapperMemory struct {
	Data  map[string]*Tx
	mutex sync.Mutex
}

func NewTxMapper() TxMapper {
	return &TxMapperMemory{
		Data:  make(map[string]*Tx),
		mutex: sync.Mutex{},
	}
}

func (tm *TxMapperMemory) AddEncryptedTx(identityPreimage []byte, encryptedTx []byte) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tx, exists := tm.Data[hex.EncodeToString(identityPreimage)]
	if !exists {
		tx = &Tx{}
		tm.Data[hex.EncodeToString(identityPreimage)] = tx
	}
	tx.EncryptedTx = encryptedTx
	return nil
}

func (tm *TxMapperMemory) AddDecryptionData(identityPreimage []byte, dd *DecryptionData) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tx, exists := tm.Data[hex.EncodeToString(identityPreimage)]
	if !exists {
		tx = &Tx{}
		tm.Data[hex.EncodeToString(identityPreimage)] = tx
	}
	tx.DD = dd
	return nil
}

func (tm *TxMapperMemory) AddBlockHash(slot uint64, blockHash common.Hash) error {
	for _, val := range tm.Data {
		if val.DD.Slot == slot {
			val.BlockHash = blockHash.Bytes()
			return nil
		}
	}
	return nil
}

func (tm *TxMapperMemory) CanBeDecrypted(identityPreimage []byte) (bool, error) {
	tx, exists := tm.Data[hex.EncodeToString(identityPreimage)]
	if !exists {
		return false, nil
	}
	return len(tx.EncryptedTx) > 0 && tx.DD != nil, nil
}

func (tm *TxMapperMemory) RemoveTx(identityPreimage []byte) error {
	if canBe, _ := tm.CanBeDecrypted(identityPreimage); !canBe {
		return errors.New("unable to remove Tx which cant be decrypted")
	}

	delete(tm.Data, hex.EncodeToString(identityPreimage))
	return nil
}
