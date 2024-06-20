package metrics

type DecryptionData struct {
	Key  []byte
	Slot uint64
}

type Tx struct {
	EncryptedTx []byte
	DD          *DecryptionData
	BlockHash   []byte
}

type ITxMapper interface {
	AddEncryptedTx(identityPreimage []byte, encryptedTx []byte) error
	AddDecryptionData(identityPreimage []byte, dd *DecryptionData) error
	CanBeDecrypted(identityPreimage []byte) (bool, error)
	AddBlockHash(slot uint64, blockHash []byte) error
}
