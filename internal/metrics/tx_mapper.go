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

func NewTxMapperMemory() TxMapper {
	return &TxMapperMemory{
		Data:  make(map[string]*Tx),
		mutex: sync.Mutex{},
	}
}

func (tm *TxMapperMemory) AddEncryptedTx(txIndex int64, eon int64, identityPreimage []byte, encryptedTx []byte) error {
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

func (tm *TxMapperMemory) AddDecryptionData(eon int64, identityPreimage []byte, dd *DecryptionData) error {
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

func (tm *TxMapperMemory) AddKeyShare(eon int64, identityPreimage []byte, keyperIndex int64, ks *KeyShare) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tx, exists := tm.Data[hex.EncodeToString(identityPreimage)]
	if !exists {
		tx = &Tx{}
		tm.Data[hex.EncodeToString(identityPreimage)] = tx
	}
	tx.KS = ks
	return nil
}

func (tm *TxMapperMemory) AddBlockHash(slot int64, blockHash common.Hash) error {
	for _, val := range tm.Data {
		if val.DD.Slot == slot {
			val.BlockHash = blockHash.Bytes()
			return nil
		}
	}
	return nil
}

func (tm *TxMapperMemory) CanBeDecrypted(txIndex int64, eon int64, identityPreimage []byte) (bool, error) {
	tx, exists := tm.Data[hex.EncodeToString(identityPreimage)]
	if !exists {
		return false, nil
	}
	return len(tx.EncryptedTx) > 0 && tx.DD != nil, nil
}

func (tm *TxMapperMemory) RemoveTx(txIndex int64, eon int64, identityPreimage []byte) error {
	if canBe, _ := tm.CanBeDecrypted(txIndex, eon, identityPreimage); !canBe {
		return errors.New("unable to remove Tx which cant be decrypted")
	}

	delete(tm.Data, hex.EncodeToString(identityPreimage))
	return nil
}
