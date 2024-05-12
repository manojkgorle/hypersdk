package auth

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	bn254 "github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/hash/mimc"
)

type Circuit struct {
	PreImage frontend.Variable
	Hash     frontend.Variable `gnark:",public"`
}

func (circuit *Circuit) Define(api frontend.API) error {
	mimc, _ := mimc.NewMiMC(api)
	mimc.Write(circuit.PreImage)
	api.AssertIsEqual(circuit.Hash, mimc.Sum())
	return nil
}

func MimcHash(data []byte) string {
	f := bn254.NewMiMC()
	f.Write(data)
	hash := f.Sum(nil)
	if len(hash) < 32 {
		padding := make([]byte, 32-len(hash))
		hash = append(hash, padding...)
	}
	hashInt := big.NewInt(0).SetBytes(hash)
	return hashInt.String()
}

// setup gives proving key and verifying key.
// proving key is similar to private key, and verifiying key is similar to public key.
func SetUp() (constraint.ConstraintSystem, groth16.ProvingKey, groth16.VerifyingKey, error) {
	var circuit Circuit
	r1cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("compile failed: %v", err)
	}

	pk, vk, err := groth16.Setup(r1cs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("setup failed: %v", err)
	}
	return r1cs, pk, vk, nil
}
