package metrics

import (
	"errors"
	"sync"
)

type DecryptionData struct {
	Key  []byte
	Slot uint64
}

type Tx struct {
	EncryptedTx []byte
	DD          *DecryptionData
}

type TxMapper struct {
	Data  map[string]*Tx
	mutex sync.Mutex
}

func NewTxMapper() *TxMapper {
	return &TxMapper{
		Data:  make(map[string]*Tx),
		mutex: sync.Mutex{},
	}
}

func (tm *TxMapper) AddEncryptedTx(identityPreimage string, encryptedTx []byte) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tx, exists := tm.Data[identityPreimage]
	if !exists {
		tx = &Tx{}
		tm.Data[identityPreimage] = tx
	}
	tx.EncryptedTx = encryptedTx
}

func (tm *TxMapper) AddDecryptionData(identityPreimage string, dd *DecryptionData) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tx, exists := tm.Data[identityPreimage]
	if !exists {
		tx = &Tx{}
		tm.Data[identityPreimage] = tx
	}
	tx.DD = dd
}

func (tm *TxMapper) CanBeDecrypted(identityPreimage string) bool {
	tx, exists := tm.Data[identityPreimage]
	if !exists {
		return false
	}
	return len(tx.EncryptedTx) > 0 && tx.DD != nil
}

func (tm *TxMapper) RemoveTx(identityPreimage string) error {
	if !tm.CanBeDecrypted(identityPreimage) {
		return errors.New("unable to remove Tx which cant be decrypted")
	}

	delete(tm.Data, identityPreimage)
	return nil
}
