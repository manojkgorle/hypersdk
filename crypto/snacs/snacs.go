package snacs

import (
	"bytes"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
)

// only helper functions for groth16 verification are added.
// this gives ability to verify any groth16 proofs instead of restricting to a circuit

func VKeyToBytes(VKey groth16.VerifyingKey) ([]byte, error) {
	var buf bytes.Buffer
	_, err := VKey.WriteRawTo(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ProofToBytes(proof groth16.Proof) ([]byte, error) {
	var buf bytes.Buffer
	_, err := proof.WriteRawTo(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func PubWitToBytes(pubWit witness.Witness) ([]byte, error) {
	var buf bytes.Buffer
	_, err := pubWit.WriteTo(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func VkeyFromBytes(data []byte) (groth16.VerifyingKey, error) {
	vKey := groth16.NewVerifyingKey(ecc.BN254)
	_, err := vKey.ReadFrom(bytes.NewReader(data))
	if err != nil {
		return vKey, err
	}
	return vKey, nil
}

func ProofFromBytes(data []byte) (groth16.Proof, error) {
	proof := groth16.NewProof(ecc.BN254)
	_, err := proof.ReadFrom(bytes.NewReader(data))
	if err != nil {
		return proof, err
	}
	return proof, nil
}

func PubWitFromBytes(data []byte) (witness.Witness, error) {
	pubWit, _ := witness.New(ecc.BN254.ScalarField())
	_, err := pubWit.ReadFrom(bytes.NewReader(data))
	if err != nil {
		return pubWit, err
	}
	return pubWit, nil
}
