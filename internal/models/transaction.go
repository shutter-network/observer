package models

type Transaction struct {
	EncryptedTx   []byte
	DecryptionKey []byte
	Slot          uint64
}
