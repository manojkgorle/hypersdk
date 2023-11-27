package actions

import (
	"context"
	"math"

	_ "github.com/manojkgorle/hyper-wasm/auth"
	"github.com/manojkgorle/hyper-wasm/storage"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	_ "github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*DeployContract)(nil)

type DeployContract struct {
	ContractCode []byte `json:"contractCode"`
}

func (*DeployContract) GetTypeID() uint8 {
	return deployContractID
}

// @todo below 3 functions are declared as part of getting registry work.
// we need to change the relevant fields later
func (*DeployContract) StateKeys(_ chain.Auth, txID ids.ID) []string {
	return []string{
		string(storage.AssetKey(txID)),
	}
}

func (*DeployContract) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.AssetChunks}
}

func (*DeployContract) OutputsWarpMessage() bool {
	return false
}

// @todo what is this, and how does this work??
func UnmarshalDeployContract(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var deploy DeployContract
	// p.UnpackBytes(MaxSymbolSize, true, &deploy.Symbol)

	// p.UnpackBytes(MaxMetadataSize, true, &deploy.Metadata)
	p.UnpackBytes(int(math.MaxUint32), true, &deploy.ContractCode)
	return &deploy, p.Err()
}

// @todo define your contract deployment logic here
// @todo create storage write functions in storage module
func (d *DeployContract) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	auth chain.Auth,
	txID ids.ID,
	_ bool) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	code := d.ContractCode
	units := uint64(codec.BytesLen(code))
	if err := storage.SetContract(ctx, mu, txID, code); err != nil {
		return false, TempComputeUnits, utils.ErrBytes(err), nil, nil
	}
	//@todo implement dyanmic gas depending on contract code size.
	//@todo partially done, for the compute units
	return true, units, nil, nil, nil
}

func (d *DeployContract) Marshal(p *codec.Packer) {

	p.PackBytes(d.ContractCode)
}

func (*DeployContract) MaxComputeUnits(chain.Rules) uint64 {
	return TempComputeUnits
}

func (d *DeployContract) Size() int {
	// TODO: add small bytes (smaller int prefix)
	return codec.BytesLen(d.ContractCode)
}

func (*DeployContract) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
