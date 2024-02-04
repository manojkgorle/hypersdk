// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"errors"

	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/ava-labs/avalanchego/ids"
)

const WarpBlockSize = consts.Uint64Len + consts.Uint64Len + consts.IDLen +
	consts.IDLen + consts.IDLen

type WarpBlock struct {
	Tmstmp    int64  `json:"timestamp"`
	Hght      uint64 `json:"height"`
	Prnt      ids.ID `json:"parent"`
	StateRoot ids.ID `json:"stateRoot"`
	// TxID is the transaction that created this message. This is used to ensure
	// there is WarpID uniqueness.
	TxID ids.ID `json:"txID"`
}

func (w *WarpBlock) Marshal() ([]byte, error) {
	p := codec.NewWriter(WarpBlockSize, WarpBlockSize)
	p.PackInt64(w.Tmstmp)
	p.PackUint64(w.Hght)
	p.PackID(w.Prnt)
	p.PackID(w.StateRoot)
	p.PackID(w.TxID)
	return p.Bytes(), p.Err()
}

func UnmarshalWarpBlock(b []byte) (*WarpBlock, error) {
	var transfer WarpBlock
	p := codec.NewReader(b, WarpBlockSize)
	transfer.Tmstmp = p.UnpackInt64(true)
	transfer.Hght = p.UnpackUint64(true)
	p.UnpackID(false, &transfer.Prnt)
	p.UnpackID(false, &transfer.StateRoot)
	p.UnpackID(false, &transfer.TxID)
	if err := p.Err(); err != nil {
		return nil, err
	}
	ErrInvalidObjectDecode := errors.New("invalid object")
	if !p.Empty() {
		return nil, ErrInvalidObjectDecode
	}

	return &transfer, nil
}
