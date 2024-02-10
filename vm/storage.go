// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/keys"
	"github.com/AnomalyFi/hypersdk/rpc"
	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"
)

// compactionOffset is used to randomize the height that we compact
// deleted blocks. This prevents all nodes on the network from deleting
// data from disk at the same time.
var compactionOffset int = -1

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	blockPrefix           = 0x0
	warpSignaturePrefix   = 0x1
	warpFetchPrefix       = 0x2
	blockCommitHashPrefix = 0x3
	publicKeyBytes        = 48
)

var (
	isSyncing           = []byte("is_syncing")
	lastAccepted        = []byte("last_accepted")
	lastBlockCommitHash = []byte("last_block_commit_hash")
	signatureLRU        = &cache.LRU[string, *chain.WarpSignature]{Size: 1024}
	blockCommitHashLRU  = &cache.LRU[string, []byte]{Size: 2048}
)

func PrefixBlockHeightKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

func (vm *VM) HasGenesis() (bool, error) {
	return vm.HasDiskBlock(0)
}

func (vm *VM) GetGenesis() (*chain.StatefulBlock, error) {
	return vm.GetDiskBlock(0)
}

func (vm *VM) GetLastBlockCommitHashHeight() (uint64, error) {
	h, err := vm.vmDB.Get(lastBlockCommitHash)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(h), nil
}

func (vm *VM) SetLastAcceptedHeight(height uint64) error {
	return vm.vmDB.Put(lastAccepted, binary.BigEndian.AppendUint64(nil, height))
}

func (vm *VM) HasLastAccepted() (bool, error) {
	return vm.vmDB.Has(lastAccepted)
}

func (vm *VM) GetLastAcceptedHeight() (uint64, error) {
	b, err := vm.vmDB.Get(lastAccepted)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b), nil
}

func (vm *VM) shouldComapct(expiryHeight uint64) bool {
	if compactionOffset == -1 {
		compactionOffset = rand.Intn(vm.config.GetBlockCompactionFrequency()) //nolint:gosec
		vm.Logger().Info("setting compaction offset", zap.Int("n", compactionOffset))
	}
	return expiryHeight%uint64(vm.config.GetBlockCompactionFrequency()) == uint64(compactionOffset)
}

// UpdateLastAccepted updates the [lastAccepted] index, stores [blk] on-disk,
// adds [blk] to the [acceptedCache], and deletes any expired blocks from
// disk.
//
// Blocks written to disk are only used when restarting the node. During normal
// operation, we only fetch blocks from memory.
//
// We store blocks by height because it doesn't cause nearly as much
// compaction as storing blocks randomly on-disk (when using [block.ID]).
func (vm *VM) UpdateLastAccepted(blk *chain.StatelessBlock) error {
	batch := vm.vmDB.NewBatch()
	if err := batch.Put(lastAccepted, binary.BigEndian.AppendUint64(nil, blk.Height())); err != nil {
		return err
	}
	if err := batch.Put(PrefixBlockHeightKey(blk.Height()), blk.Bytes()); err != nil {
		return err
	}
	expiryHeight := blk.Height() - uint64(vm.config.GetAcceptedBlockWindow())
	var expired bool
	if expiryHeight > 0 && expiryHeight < blk.Height() { // ensure we don't free genesis
		if err := batch.Delete(PrefixBlockHeightKey(expiryHeight)); err != nil {
			return err
		}
		expired = true
		vm.metrics.deletedBlocks.Inc()
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("%w: unable to update last accepted", err)
	}
	vm.lastAccepted = blk
	vm.acceptedBlocksByID.Put(blk.ID(), blk)
	vm.acceptedBlocksByHeight.Put(blk.Height(), blk.ID())
	if expired && vm.shouldComapct(expiryHeight) {
		go func() {
			start := time.Now()
			if err := vm.CompactDiskBlocks(expiryHeight); err != nil {
				vm.Logger().Error("unable to compact blocks", zap.Error(err))
				return
			}
			vm.Logger().Info("compacted disk blocks", zap.Uint64("end", expiryHeight), zap.Duration("t", time.Since(start)))
		}()
	}
	return nil
}

func (vm *VM) GetDiskBlock(height uint64) (*chain.StatefulBlock, error) {
	b, err := vm.vmDB.Get(PrefixBlockHeightKey(height))
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalBlock(b, vm)
}

func (vm *VM) HasDiskBlock(height uint64) (bool, error) {
	return vm.vmDB.Has(PrefixBlockHeightKey(height))
}

// CompactDiskBlocks forces compaction on the entire range of blocks up to [lastExpired].
//
// This can be used to ensure we clean up all large tombstoned keys on a regular basis instead
// of waiting for the database to run a compaction (and potentially delete GBs of data at once).
func (vm *VM) CompactDiskBlocks(lastExpired uint64) error {
	return vm.vmDB.Compact([]byte{blockPrefix}, PrefixBlockHeightKey(lastExpired))
}

func (vm *VM) GetDiskIsSyncing() (bool, error) {
	v, err := vm.vmDB.Get(isSyncing)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v[0] == 0x1, nil
}

func (vm *VM) PutDiskIsSyncing(v bool) error {
	if v {
		return vm.vmDB.Put(isSyncing, []byte{0x1})
	}
	return vm.vmDB.Put(isSyncing, []byte{0x0})
}

func (vm *VM) GetOutgoingWarpMessage(txID ids.ID) (*warp.UnsignedMessage, error) {
	if txID[0] == blockCommitHashPrefix {
		height := binary.BigEndian.Uint64(txID[1:9])
		b, err := vm.GetProcessedBlockCommitHash(height)
		if err != nil {
			return nil, err
		}
		return warp.ParseUnsignedMessage(b)
	}

	p := vm.c.StateManager().OutgoingWarpKeyPrefix(txID)
	k := keys.EncodeChunks(p, chain.MaxOutgoingWarpChunks)
	vs, errs := vm.ReadState(context.TODO(), [][]byte{k})
	v, err := vs[0], errs[0]
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return warp.ParseUnsignedMessage(v)
}

func PrefixWarpSignatureKey(txID ids.ID, signer *bls.PublicKey) []byte {
	k := make([]byte, 1+consts.IDLen+bls.PublicKeyLen)
	k[0] = warpSignaturePrefix
	copy(k[1:], txID[:])
	copy(k[1+consts.IDLen:], bls.PublicKeyToBytes(signer))
	return k
}

func (vm *VM) StoreWarpSignature(txID ids.ID, signer *bls.PublicKey, signature []byte) error {
	k := PrefixWarpSignatureKey(txID, signer)
	// Cache any signature we produce for later queries from peers
	if bytes.Equal(vm.pkBytes, bls.PublicKeyToBytes(signer)) {
		signatureLRU.Put(string(k), chain.NewWarpSignature(vm.pkBytes, signature))
	}
	return vm.vmDB.Put(k, signature)
}

func (vm *VM) GetWarpSignature(txID ids.ID, signer *bls.PublicKey) (*chain.WarpSignature, error) {
	k := PrefixWarpSignatureKey(txID, signer)
	if ws, ok := signatureLRU.Get(string(k)); ok {
		return ws, nil
	}
	v, err := vm.vmDB.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ws := &chain.WarpSignature{
		PublicKey: bls.PublicKeyToBytes(signer),
		Signature: v,
	}
	return ws, nil
}

func (vm *VM) GetWarpSignatures(txID ids.ID) ([]*chain.WarpSignature, error) {
	prefix := make([]byte, 1+consts.IDLen)
	prefix[0] = warpSignaturePrefix
	copy(prefix[1:], txID[:])
	iter := vm.vmDB.NewIteratorWithPrefix(prefix)
	defer iter.Release()

	// Collect all signatures we have for a txID
	signatures := []*chain.WarpSignature{}
	for iter.Next() {
		k := iter.Key()
		signatures = append(signatures, &chain.WarpSignature{
			PublicKey: k[len(k)-bls.PublicKeyLen:],
			Signature: iter.Value(),
		})
	}
	return signatures, iter.Error()
}

func PrefixWarpFetchKey(txID ids.ID) []byte {
	k := make([]byte, 1+consts.IDLen)
	k[0] = warpFetchPrefix
	copy(k[1:], txID[:])
	return k
}

func (vm *VM) StoreWarpFetch(txID ids.ID) error {
	k := PrefixWarpFetchKey(txID)
	return vm.vmDB.Put(k, binary.BigEndian.AppendUint64(nil, uint64(time.Now().UnixMilli())))
}

func (vm *VM) GetWarpFetch(txID ids.ID) (int64, error) {
	k := PrefixWarpFetchKey(txID)
	v, err := vm.vmDB.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return -1, nil
	}
	if err != nil {
		return -1, err
	}
	return int64(binary.BigEndian.Uint64(v)), nil
}

// to ensure compatibility with warpManager.go; we prefix prefixed key with prefixWarpSignatureKey
func PrefixBlockCommitHashKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockCommitHashPrefix
	binary.BigEndian.PutUint64(k, height)
	return k
}

func ToID(key []byte) ids.ID {
	k := ids.ID{}
	copy(k[1:9], key[:])
	return k
}

func PackValidatorsData(initBytes []byte, PublicKey *bls.PublicKey, weight uint64) []byte {
	pbKeyBytes := bls.PublicKeyToBytes(PublicKey)
	return append(initBytes, binary.BigEndian.AppendUint64(pbKeyBytes, weight)...)
}

func (vm *VM) StoreUnprocessedBlockCommitHash(height uint64, pHeight uint64, stateRoot ids.ID) {
	k := PrefixBlockCommitHashKey(height)
	h := binary.BigEndian.AppendUint64(stateRoot[:], pHeight)
	if err := vm.vmDB.Put(k, h); err != nil {
		vm.Fatal("Could not store unprocessed block commit hash", zap.Error(err))
	}
}

func (vm *VM) GetUnprocessedBlockCommitHash(height uint64) (uint64, ids.ID, error) {
	k := PrefixBlockCommitHashKey(height)
	v, err := vm.vmDB.Get(k)
	if err != nil {
		return 0, ids.Empty, err
	}
	vID, err := ids.ToID(v[:32])
	if err != nil {
		return 0, ids.Empty, err
	}
	pHeight := binary.BigEndian.Uint64(v[32:])
	return pHeight, vID, nil
}

func (vm *VM) StoreProcessedBlockCommitHash(height uint64, stateRoot []byte) {
	k := PrefixBlockCommitHashKey(height)
	if err := vm.vmDB.Put(k, stateRoot[:]); err != nil {
		vm.Fatal("Could not store processed block commit hash", zap.Error(err))
	}
}

func (vm *VM) GetProcessedBlockCommitHash(height uint64) ([]byte, error) {
	k := PrefixBlockCommitHashKey(height)
	return vm.vmDB.Get(k)
}

func (vm *VM) StoreBlockCommitHash(height uint64, pBlockHeight uint64, stateRoot ids.ID) error {
	// err handling cases:
	// vdrState access error: dont panic & quit recover
	// cant sign or cant store panic & throuw error for res to handle
	lh, err := vm.GetLastBlockCommitHashHeight()
	if err != nil {
		vm.Logger().Error("could not get last block commit hash", zap.Error(err))
		return err
	}

	if height != lh+1 {
		pHeight, hash, err := vm.GetUnprocessedBlockCommitHash(height - 1)
		if err != nil {
			vm.Logger().Error("could not retrieve last unprocessed block hash", zap.Error(err))
		} else {
			if err := vm.innerStoreBlockCommitHash(height-1, pHeight, hash); err != nil {
				if errors.Is(err, ErrAccesingVdrState) {
					return nil
				} //@todo concrete testing needed for error handling
				return fmt.Errorf("error in mid clearance of block commit hash queue")
			}
		}
	}
	if err := vm.innerStoreBlockCommitHash(height, pBlockHeight, stateRoot); err != nil {
		if errors.Is(err, ErrAccesingVdrState) {
			return nil
		} else {
			return err
		}
	}
	return nil
}

func (vm *VM) innerStoreBlockCommitHash(height uint64, pBlkHeight uint64, stateRoot ids.ID) error {
	// -- make a new function @todo
	vdrSet, err := vm.snowCtx.ValidatorState.GetValidatorSet(context.Background(), pBlkHeight, vm.SubnetID())
	if err != nil {
		vm.Logger().Error("could not access validator set", zap.Error(err))
		vm.StoreUnprocessedBlockCommitHash(height, pBlkHeight, stateRoot)
		return ErrAccesingVdrState
	}
	validators, _, err := rpc.GetCanonicalValidatorSet(context.Background(), vdrSet)
	if err != nil {
		vm.Logger().Error("unable to get canonical validator set", zap.Error(err))
		vm.StoreUnprocessedBlockCommitHash(height, pBlkHeight, stateRoot)
		return ErrCanonicalOrdering
	}
	// -- till here @todo
	// Pack public keys & weight of individual validators as given in the canonical validator set
	validatorDataBytes := make([]byte, len(validators)*(publicKeyBytes+consts.Uint64Len))
	for _, validator := range validators {
		nVdrDataBytes := PackValidatorsData(validatorDataBytes, validator.PublicKey, validator.Weight)
		validatorDataBytes = append(validatorDataBytes, nVdrDataBytes...)
	}
	// hashing to scale for any number of validators. warp messaging has a msg size limit of 256 KiB
	vdrDataBytesHash := crypto.Keccak256(validatorDataBytes)
	k := PrefixBlockCommitHashKey(height) // key-size: 9 bytes
	// prefix k with warpPrefixKey to ensure compatiblity with warpManager.go for gossip of warp messages, to ensure minimal relayer build
	keyPrefixed := ToID(k)

	msg := append(vdrDataBytesHash, stateRoot[:]...)
	unSigMsg, err := warp.NewUnsignedMessage(vm.NetworkID(), vm.ChainID(), msg)
	if err != nil {
		vm.Fatal("unable to create new unsigned warp message", zap.Error(err))
	}
	vm.StoreProcessedBlockCommitHash(height, unSigMsg.Bytes())
	signedMsg, err := vm.snowCtx.WarpSigner.Sign(unSigMsg)
	if err != nil {
		vm.Fatal("unable to sign block commit hash", zap.Error(err), zap.Uint64("height:", height))
	}
	if err := vm.StoreWarpSignature(keyPrefixed, vm.snowCtx.PublicKey, signedMsg); err != nil {
		vm.Fatal("unable to store block commit hash", zap.Error(err), zap.Uint64("height:", height))
	}
	if err := vm.vmDB.Put(lastBlockCommitHash, binary.BigEndian.AppendUint64(nil, height)); err != nil {
		return err //@todo fatal or a soft error
	}
	blockCommitHashLRU.Put(string(k), signedMsg)

	vm.webSocketServer.SendBlockCommitHash(signedMsg, height, pBlkHeight, keyPrefixed) // transmit signature to listeners i.e. relayer
	vm.warpManager.GatherSignatures(context.TODO(), keyPrefixed, unSigMsg.Bytes())     // transmit as a warp message
	vm.Logger().Info("stored block commit hash", zap.Uint64("block height", height))
	return nil
}
