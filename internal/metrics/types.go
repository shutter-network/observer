package metrics

import "github.com/ethereum/go-ethereum/common"

type DecryptionData struct {
	Key  []byte
	Slot int64
}

type KeyShare struct {
	Share []byte
	Slot  int64
}

type Tx struct {
	EncryptedTx []byte
	DD          *DecryptionData
	KS          *KeyShare
	BlockHash   []byte
}

type TxMapper interface {
	AddEncryptedTx(txIndex int64, eon int64, identityPreimage []byte, encryptedTx []byte) error
	AddDecryptionData(eon int64, identityPreimage []byte, dd *DecryptionData) error
	AddKeyShare(eon int64, identityPreimage []byte, keyperIndex int64, ks *KeyShare) error
	CanBeDecrypted(txIndex int64, eon int64, identityPreimage []byte) (bool, error)
	AddBlockHash(slot int64, blockHash common.Hash) error
}
