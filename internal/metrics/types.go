package metrics

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

type ITxMapper interface {
	AddEncryptedTx(identityPreimage []byte, encryptedTx []byte) error
	AddDecryptionData(identityPreimage []byte, dd *DecryptionData) error
	AddKeyShare(identityPreimage []byte, ks *KeyShare) error
	CanBeDecrypted(identityPreimage []byte) (bool, error)
	AddBlockHash(slot uint64, blockHash []byte) error
}
