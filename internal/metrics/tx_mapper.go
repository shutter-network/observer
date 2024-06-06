package metrics

import "sync"

type TxMetrics interface {
	AddEncryptedTx(identity string, encryptedTx []byte)
	AddDecryptionData(identity string, dd *DecryptionData)
	HasCompleteTx(identity string) bool
}

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

func (tm *TxMapper) AddEncryptedTx(identity string, encryptedTx []byte) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tx, exists := tm.Data[identity]
	if !exists {
		tx = &Tx{}
		tm.Data[identity] = tx
	}
	tx.EncryptedTx = encryptedTx
}

func (tm *TxMapper) AddDecryptionData(identity string, dd *DecryptionData) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tx, exists := tm.Data[identity]
	if !exists {
		tx = &Tx{}
		tm.Data[identity] = tx
	}
	tx.DD = dd
}

func (tm *TxMapper) HasCompleteTx(identity string) bool {
	tx, exists := tm.Data[identity]
	if !exists {
		return false
	}
	return len(tx.EncryptedTx) > 0 && tx.DD != nil
}
