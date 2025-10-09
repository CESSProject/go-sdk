package retriever

import (
	"encoding/json"

	"github.com/CESSProject/cess-crypto/gosdk"
	"github.com/ChainSafe/go-schnorrkel"
	"github.com/gtank/ristretto255"
	"github.com/pkg/errors"
)

// ReEncryptKey performs proxy re-encryption on a capsule using a re-encryption key.
// This transforms the original capsule into a new version decryptable by the target party.
//
// Parameters:
//
//	capsule - Original encrypted capsule bytes (JSON marshaled)
//	rkb     - Re-encryption key bytes (ristretto255 scalar serialization)
//
// Returns:
//
//	[]byte - New re-encrypted capsule bytes (JSON marshaled)
//	error  - Possible errors include:
//	           - Invalid capsule format
//	           - Invalid re-encryption key format
//	           - Re-encryption operation failure
//	           - Result serialization failure
func ReEncryptKey(capsule, rkb []byte) ([]byte, error) {
	var (
		c   gosdk.Capsule
		rk  *ristretto255.Scalar
		err error
	)
	if err = json.Unmarshal(capsule, &c); err != nil {
		return nil, errors.Wrap(err, "re-encrypt key error")
	}
	rk = ristretto255.NewScalar()
	if err = rk.UnmarshalText(rkb); err != nil {
		return nil, errors.Wrap(err, "re-encrypt pre key error")
	}
	newCapsule, err := gosdk.ReEncryptKey(rk, &c)
	if err != nil {
		return nil, errors.Wrap(err, "re-encrypt pre key error")
	}
	newCapsuleBytes, err := json.Marshal(newCapsule)
	if err != nil {
		return nil, errors.Wrap(err, "re-encrypt pre key error")
	}
	return newCapsuleBytes, nil
}

// DecryptReKey decrypts re-encrypted key material using recipient's secret derived from mnemonic.
// This enables the target party to access the original encrypted data through proxy re-encryption.
//
// Parameters:
//
//	mnemonic   - Recipient's mnemonic phrase for secret derivation
//	pkX        - Public key X bytes used in re-encryption (32-byte expected)
//	newCapsule - Re-encrypted capsule bytes (JSON marshaled)
//
// Returns:
//
//	[]byte - Decrypted symmetric key bytes
//	error  - Possible errors include:
//	           - Invalid public key X format
//	           - Public key deserialization failure
//	           - Invalid capsule format
//	           - Invalid mnemonic phrase
//	           - Key decryption operation failure
func DecryptReKey(mnemonic string, pkX, newCapsule []byte) ([]byte, error) {
	var (
		X  [32]byte
		nc gosdk.Capsule
	)
	if len(pkX) < 32 {
		return nil, errors.Wrap(errors.New("bad pubkey X"), "decrypt re-key error")
	}
	copy(X[:], pkX[:32])
	pubkeyX, err := schnorrkel.NewPublicKey(X)
	if err != nil {
		return nil, errors.Wrap(err, "decrypt re-key error")
	}
	if err = json.Unmarshal(newCapsule, &nc); err != nil {
		return nil, errors.Wrap(err, "decrypt re-key error")
	}
	ms, err := schnorrkel.MiniSecretKeyFromMnemonic(mnemonic, "")
	if err != nil {
		return nil, errors.Wrap(err, "decrypt re-key error")
	}
	key, err := gosdk.DecryptReKey(ms.ExpandEd25519(), &nc, pubkeyX)
	if err != nil {
		return nil, errors.Wrap(err, "decrypt re-key error")
	}
	return key, nil
}

// DecryptKey retrieves the original symmetric encryption key using the data owner's mnemonic.
// This operates on the original (non-re-encrypted) capsule structure.
//
// Parameters:
//
//	mnemonic - Data owner's mnemonic phrase for secret derivation
//	capsule  - Original encrypted capsule bytes (JSON marshaled)
//
// Returns:
//
//	[]byte - Decrypted symmetric key bytes
//	error  - Possible errors include:
//	           - Invalid capsule format
//	           - Invalid mnemonic phrase
//	           - Key decryption operation failure
func DecryptKey(mnemonic string, capsule []byte) ([]byte, error) {
	var (
		c   gosdk.Capsule
		err error
	)
	if err = json.Unmarshal(capsule, &c); err != nil {
		return nil, errors.Wrap(err, "decrypt key error")
	}
	ms, err := schnorrkel.MiniSecretKeyFromMnemonic(mnemonic, "")
	if err != nil {
		return nil, errors.Wrap(err, "decrypt key error")
	}
	key, err := gosdk.DecryptKey(ms.ExpandEd25519(), &c)
	if err != nil {
		return nil, errors.Wrap(err, "decrypt key error")
	}
	return key, nil
}

// GenReEncryptionKey generates a re-encryption key and corresponding public key using Schnorrkel scheme.
// This implements proxy re-encryption mechanism for decentralized storage systems.
//
// Parameters:
//
//	mnemonic - User's mnemonic phrase for key derivation
//	pkB      - Recipient's public key bytes (32-byte expected)
//
// Returns:
//
//	[]byte - Marshaled re-encryption key (rk)
//	[]byte - Encoded public key bytes for encryption (pkX)
//	error  - Possible errors include:
//	           - Invalid mnemonic phrase
//	           - Public key deserialization failure
//	           - Re-encryption key generation failure
//	           - Key serialization failure
func GenReEncryptionKey(mnemonic string, pkB []byte) ([]byte, []byte, error) {
	if len(pkB) != 32 {
		return nil, nil, errors.Wrap(errors.New("public key length error"), "generate re-encryption key error")
	}
	ms, err := schnorrkel.MiniSecretKeyFromMnemonic(mnemonic, "")
	if err != nil {
		return nil, nil, errors.Wrap(err, "generate re-encryption key error")
	}
	var pkbArray [32]byte
	copy(pkbArray[:], pkB[:32])
	pubkeyB, err := schnorrkel.NewPublicKey(pkbArray)
	if err != nil {
		return nil, nil, errors.Wrap(err, "generate re-encryption key error")
	}
	rk, pkX, err := gosdk.GenReKey(ms.ExpandEd25519(), pubkeyB)
	if err != nil {
		return nil, nil, errors.Wrap(err, "generate re-encryption key error")
	}
	txtRk, err := rk.MarshalText()
	if err != nil {
		return nil, nil, errors.Wrap(err, "generate re-encryption key error")
	}
	pkxArray := pkX.Encode()
	return txtRk, pkxArray[:], nil
}
