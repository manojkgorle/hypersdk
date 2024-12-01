// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*DA)(nil)

type DA struct {
	DAData []byte `json:"data"`
}

func (*DA) GetTypeID() uint8 {
	return mconsts.DAID
}

func (*DA) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{}
}

func (*DA) StateKeysMaxChunks() []uint16 {
	return []uint16{}
}

func (*DA) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) (bool, uint64, []byte, error) {
	return true, 1, nil, nil
}

func (*DA) MaxComputeUnits(chain.Rules) uint64 {
	return DAComputeUnits
}

func (d *DA) Size() int {
	return codec.BytesLen(d.DAData)
}

func (d *DA) Marshal(p *codec.Packer) {
	p.PackBytes(d.DAData)
}

func UnmarshalDA(p *codec.Packer) (chain.Action, error) {
	var da DA
	p.UnpackBytes(-1, true, &da.DAData) // we do not verify the typeID is valid
	if err := p.Err(); err != nil {
		return nil, err
	}
	return &da, nil
}

func (*DA) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (d *DA) Data() []byte {
	return d.DAData
}
