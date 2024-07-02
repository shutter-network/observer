package metrics

import "github.com/ethereum/go-ethereum/common"

type DecryptionData struct {
	Key  []byte
	Slot uint64
}

type KeyShare struct {
	Share []byte
	Slot  uint64
}

type Tx struct {
	EncryptedTx []byte
	DD          *DecryptionData
	KS          *KeyShare
	BlockHash   []byte
}

type TxMapper interface {
	AddEncryptedTx(identityPreimage []byte, encryptedTx []byte) error
	AddDecryptionData(identityPreimage []byte, dd *DecryptionData) error
	AddKeyShare(identityPreimage []byte, ks *KeyShare) error
	CanBeDecrypted(identityPreimage []byte) (bool, error)
	AddBlockHash(slot uint64, blockHash common.Hash) error
}
