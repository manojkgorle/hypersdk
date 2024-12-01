// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"fmt"
	"net/http"

	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/gorilla/rpc/v2"
	"github.com/manojkgorle/rsmt2d"
)

func NewJSONRPCHandler(
	name string,
	service interface{},
) (http.Handler, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json.NewCodec(), "application/json")
	server.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	return server, server.RegisterService(service, name)
}

func computeRowProofs(eds *rsmt2d.ExtendedDataSquare, rowIdx uint, colIdx uint) ([]byte, [][]byte, uint, uint, error) {
	tree := rsmt2d.NewDefaultTree(rsmt2d.Row, rowIdx)
	data := eds.Row(rowIdx)

	for i := uint(0); i < eds.Width(); i++ {
		err := tree.Push(data[i])
		if err != nil {
			return nil, nil, 0, 0, err
		}
	}

	merkleRoot, proof, proofIndex, numLeaves := treeProve(tree.(*rsmt2d.DefaultTree), int(colIdx))
	// tree.(*rsmt2d.DefaultTree).Prove()
	return merkleRoot, proof, uint(proofIndex), uint(numLeaves), nil
}

func treeProve(d *rsmt2d.DefaultTree, idx int) (merkleRoot []byte, proofSet [][]byte, proofIndex uint64, numLeaves uint64) {
	if err := d.Tree.SetIndex(uint64(idx)); err != nil {
		panic(fmt.Sprintf("don't call prove on a already used tree: %v", err))
	}
	for _, l := range d.Leaves() {
		d.Tree.Push(l)
	}
	return d.Tree.Prove()
}

func computeColProofs(eds *rsmt2d.ExtendedDataSquare, rowIdx uint, colIdx uint) ([]byte, [][]byte, uint, uint, error) {
	tree := rsmt2d.NewDefaultTree(rsmt2d.Col, colIdx)
	data := eds.Col(colIdx)

	for i := uint(0); i < eds.Width(); i++ {
		err := tree.Push(data[i])
		if err != nil {
			return nil, nil, 0, 0, err
		}
	}

	merkleRoot, proof, proofIndex, numLeaves := treeProve(tree.(*rsmt2d.DefaultTree), int(rowIdx))
	return merkleRoot, proof, uint(proofIndex), uint(numLeaves), nil
}
