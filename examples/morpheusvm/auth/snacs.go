package auth

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/snacs"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
)

var _ chain.Auth = (*SNACS)(nil)

const (
	SNACSComputeUnits = 10
)

type SNACS struct {
	VKey  groth16.VerifyingKey `json:"vkey,omitempty"`
	Proof groth16.Proof        `json:"proof"`
	// PubWitness witness.Witness      `json:"pub_witness"`
	addr codec.Address
}

func (s *SNACS) address() codec.Address {
	if s.addr == codec.EmptyAddress {
		s.addr = NewSNACSAddress(s.VKey)
	}
	return s.addr
}

func (*SNACS) GetTypeID() uint8 {
	return consts.SNACSID
}

func (*SNACS) ComputeUnits(chain.Rules) uint64 {
	return SNACSComputeUnits
}

func (*SNACS) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (s *SNACS) Verify(_ context.Context, msg []byte) error {
	msgHash := sha256.Sum224(msg)
	hash := MimcHash(msgHash[:])
	assignement := &Circuit{
		PreImage: frontend.Variable(msgHash[:]), // preImage can be anything, as PreImage is not public input. But filling the field is necessary for the witness to be created
		Hash:     frontend.Variable(hash),
	}
	witness, err := frontend.NewWitness(assignement, ecc.BN254.ScalarField())
	if err != nil {
		return err
	}
	pubWit, err := witness.Public()
	if err != nil {
		return err
	}
	return groth16.Verify(s.Proof, s.VKey, pubWit)
}

func (s *SNACS) Actor() codec.Address {
	return s.address()
}

func (s *SNACS) Sponsor() codec.Address {
	return s.address()
}

func (s *SNACS) Size() int {
	vKeyBytes, _ := snacs.VKeyToBytes(s.VKey)
	proofBytes, _ := snacs.ProofToBytes(s.Proof)

	return len(vKeyBytes) + len(proofBytes)
}

func (s *SNACS) Marshal(p *codec.Packer) {
	vKeyBytes, _ := snacs.VKeyToBytes(s.VKey)
	proofBytes, _ := snacs.ProofToBytes(s.Proof)

	p.PackBytes(vKeyBytes)
	p.PackBytes(proofBytes)

}

func UnmarshalSNACS(p *codec.Packer) (chain.Auth, error) {
	var s SNACS
	var vKeyBytes []byte
	p.UnpackBytes(-1, true, &vKeyBytes)
	vKey, err := snacs.VkeyFromBytes(vKeyBytes)
	if err != nil {
		return nil, err
	}
	s.VKey = vKey
	var proofBytes []byte
	p.UnpackBytes(-1, true, &proofBytes)
	s.Proof, err = snacs.ProofFromBytes(proofBytes)
	if err != nil {
		return nil, err
	}
	return &s, p.Err()
}

var _ chain.AuthFactory = (*SNACSFactory)(nil)

type SNACSFactory struct {
	PKey groth16.ProvingKey
	VKey groth16.VerifyingKey
	CS   constraint.ConstraintSystem
}

func NewSNACSFactory(pKey groth16.ProvingKey, vKey groth16.VerifyingKey, cs constraint.ConstraintSystem) *SNACSFactory {
	return &SNACSFactory{PKey: pKey, VKey: vKey, CS: cs}
}

func SetUpSNACSFactory() (*SNACSFactory, error) {
	cs, pk, vk, err := SetUp()
	if err != nil {
		return nil, err
	}
	return &SNACSFactory{PKey: pk, VKey: vk, CS: cs}, nil
}

func (s *SNACSFactory) Sign(msg []byte) (chain.Auth, error) {
	msgHash := sha256.Sum224(msg)
	hash := MimcHash(msgHash[:])
	assignment := &Circuit{
		PreImage: frontend.Variable(msgHash[:]),
		Hash:     frontend.Variable(hash),
	}

	witness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		return nil, fmt.Errorf("error creating new witness: %s", err)
	}

	proof, err := groth16.Prove(s.CS, s.PKey, witness)
	if err != nil {
		return nil, fmt.Errorf("error generating proof: %s", err)
	}

	return &SNACS{Proof: proof, VKey: s.VKey}, nil
}

func (s *SNACSFactory) MaxUnits() (uint64, uint64) {
	return uint64(hconsts.MaxUint16), SNACSComputeUnits
}

func NewSNACSAddress(Vkey groth16.VerifyingKey) codec.Address {
	vKeyBytes, _ := snacs.VKeyToBytes(Vkey)
	return codec.CreateAddress(consts.SNACSID, utils.ToID(vKeyBytes))
}
