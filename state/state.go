// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"
)

type Base interface {
	PrefetchPaths(keys [][]byte) error
}

type Immutable interface {
	GetValue(ctx context.Context, key []byte) (value []byte, err error)
}

type Mutable interface {
	Immutable

	Insert(ctx context.Context, key []byte, value []byte) error
	Remove(ctx context.Context, key []byte) error
}

type View interface {
	Immutable

	NewView(ctx context.Context, changes merkledb.ViewChanges) (merkledb.TrieView, error)
	GetMerkleRoot(ctx context.Context) (ids.ID, error)
}
