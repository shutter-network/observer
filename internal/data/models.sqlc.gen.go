// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0

package data

import (
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type TxStatusVal string

const (
	TxStatusValIncluded    TxStatusVal = "included"
	TxStatusValNotincluded TxStatusVal = "not included"
)

func (e *TxStatusVal) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = TxStatusVal(s)
	case string:
		*e = TxStatusVal(s)
	default:
		return fmt.Errorf("unsupported scan type for TxStatusVal: %T", src)
	}
	return nil
}

type NullTxStatusVal struct {
	TxStatusVal TxStatusVal
	Valid       bool // Valid is true if TxStatusVal is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullTxStatusVal) Scan(value interface{}) error {
	if value == nil {
		ns.TxStatusVal, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.TxStatusVal.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullTxStatusVal) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.TxStatusVal), nil
}

type ValidatorRegistrationValidity string

const (
	ValidatorRegistrationValidityValid            ValidatorRegistrationValidity = "valid"
	ValidatorRegistrationValidityInvalidmessage   ValidatorRegistrationValidity = "invalid message"
	ValidatorRegistrationValidityInvalidsignature ValidatorRegistrationValidity = "invalid signature"
)

func (e *ValidatorRegistrationValidity) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = ValidatorRegistrationValidity(s)
	case string:
		*e = ValidatorRegistrationValidity(s)
	default:
		return fmt.Errorf("unsupported scan type for ValidatorRegistrationValidity: %T", src)
	}
	return nil
}

type NullValidatorRegistrationValidity struct {
	ValidatorRegistrationValidity ValidatorRegistrationValidity
	Valid                         bool // Valid is true if ValidatorRegistrationValidity is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullValidatorRegistrationValidity) Scan(value interface{}) error {
	if value == nil {
		ns.ValidatorRegistrationValidity, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.ValidatorRegistrationValidity.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullValidatorRegistrationValidity) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.ValidatorRegistrationValidity), nil
}

type Block struct {
	BlockHash      []byte
	BlockNumber    int64
	BlockTimestamp int64
	TxHash         []byte
	CreatedAt      pgtype.Timestamptz
	UpdatedAt      pgtype.Timestamptz
	Slot           int64
}

type DecryptedTx struct {
	Slot                        int64
	TxIndex                     int64
	TxHash                      []byte
	TxStatus                    TxStatusVal
	CreatedAt                   pgtype.Timestamptz
	UpdatedAt                   pgtype.Timestamptz
	DecryptionKeyID             pgtype.Int8
	TransactionSubmittedEventID pgtype.Int8
}

type DecryptionKey struct {
	Eon              int64
	IdentityPreimage []byte
	Key              []byte
	CreatedAt        pgtype.Timestamptz
	UpdatedAt        pgtype.Timestamptz
	ID               int64
}

type DecryptionKeyShare struct {
	Eon                int64
	IdentityPreimage   []byte
	KeyperIndex        int64
	DecryptionKeyShare []byte
	Slot               int64
	CreatedAt          pgtype.Timestamptz
	UpdatedAt          pgtype.Timestamptz
}

type DecryptionKeysMessage struct {
	Slot       int64
	InstanceID int64
	Eon        int64
	TxPointer  int64
	CreatedAt  pgtype.Timestamptz
	UpdatedAt  pgtype.Timestamptz
}

type DecryptionKeysMessageDecryptionKey struct {
	DecryptionKeysMessageSlot int64
	KeyIndex                  int64
	CreatedAt                 pgtype.Timestamptz
	UpdatedAt                 pgtype.Timestamptz
	DecryptionKeyID           int64
}

type ProposerDuty struct {
	ID             int32
	PublicKey      string
	ValidatorIndex int64
	Slot           int64
	CreatedAt      pgtype.Timestamptz
	UpdatedAt      pgtype.Timestamptz
}

type TransactionSubmittedEvent struct {
	EventBlockHash       []byte
	EventBlockNumber     int64
	EventTxIndex         int64
	EventLogIndex        int64
	Eon                  int64
	TxIndex              int64
	IdentityPrefix       []byte
	Sender               []byte
	EncryptedTransaction []byte
	CreatedAt            pgtype.Timestamptz
	UpdatedAt            pgtype.Timestamptz
	ID                   int64
}

type ValidatorRegistrationMessage struct {
	ID                       int32
	Version                  pgtype.Int8
	ChainID                  pgtype.Int8
	ValidatorRegistryAddress []byte
	ValidatorIndex           pgtype.Int8
	Nonce                    pgtype.Int8
	IsRegisteration          pgtype.Bool
	Signature                []byte
	EventBlockNumber         int64
	EventTxIndex             int64
	EventLogIndex            int64
	Validity                 ValidatorRegistrationValidity
	CreatedAt                pgtype.Timestamptz
	UpdatedAt                pgtype.Timestamptz
}

type ValidatorRegistryEventsSyncedUntil struct {
	EnforceOneRow bool
	BlockNumber   int64
}

type ValidatorStatus struct {
	ID             int32
	ValidatorIndex pgtype.Int8
	Status         string
	CreatedAt      pgtype.Timestamptz
	UpdatedAt      pgtype.Timestamptz
}
