// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bytes"
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	brpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
)

var _ cli.Controller = (*Controller)(nil)

type Handler struct {
	h *cli.Handler
}

func NewHandler(h *cli.Handler) *Handler {
	return &Handler{h}
}

func (h *Handler) Root() *cli.Handler {
	return h.h
}

func (h *Handler) DefaultActor() (
	ids.ID, *cli.PrivateKey, chain.AuthFactory,
	*rpc.JSONRPCClient, *brpc.JSONRPCClient, *rpc.WebSocketClient, error,
) {
	addr, priv, err := h.h.GetDefaultKey(true)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	var factory chain.AuthFactory
	switch addr[0] {
	case consts.ED25519ID:
		factory = auth.NewED25519Factory(ed25519.PrivateKey(priv))
	case consts.SECP256R1ID:
		factory = auth.NewSECP256R1Factory(secp256r1.PrivateKey(priv))
	case consts.BLSID:
		p, err := bls.PrivateKeyFromBytes(priv)
		if err != nil {
			return ids.Empty, nil, nil, nil, nil, nil, err
		}
		factory = auth.NewBLSFactory(p)
	default:
		return ids.Empty, nil, nil, nil, nil, nil, ErrInvalidAddress
	}
	chainID, uris, err := h.h.GetDefaultChain(true)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	jcli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := jcli.Network(context.TODO())
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	ws, err := rpc.NewWebSocketClient(uris[0], rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	// For [defaultActor], we always send requests to the first returned URI.
	return chainID, &cli.PrivateKey{
			Address: addr,
			Bytes:   priv,
		}, factory, jcli,
		brpc.NewJSONRPCClient(
			uris[0],
			networkID,
			chainID,
		), ws, nil
}

func (h *Handler) SnackActor() (
	ids.ID, *cli.PrivateKey, chain.AuthFactory,
	*rpc.JSONRPCClient, *brpc.JSONRPCClient, *rpc.WebSocketClient, error,
) {
	pKeyBytes, err := h.h.GetDefault(snacksPK)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	pKey := groth16.NewProvingKey(ecc.BN254)
	_, err = pKey.ReadFrom(bytes.NewBuffer(pKeyBytes))
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}

	vKeyBytes, err := h.h.GetDefault(snacksVK)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	vKey := groth16.NewVerifyingKey(ecc.BN254)
	_, err = vKey.ReadFrom(bytes.NewBuffer(vKeyBytes))
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}

	csBytes, err := h.h.GetDefault(snacksCS)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	cs := groth16.NewCS(ecc.BN254)
	_, err = cs.ReadFrom(bytes.NewBuffer(csBytes))
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}

	factory := auth.NewSNACSFactory(pKey, vKey, cs)

	chainID, uris, err := h.h.GetDefaultChain(true)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	jcli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := jcli.Network(context.TODO())
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	ws, err := rpc.NewWebSocketClient(uris[0], rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	snacs := auth.SNACS{
		VKey: factory.VKey,
	}
	// For [defaultActor], we always send requests to the first returned URI.
	return chainID, &cli.PrivateKey{
			Address: snacs.Actor(),
		}, factory, jcli,
		brpc.NewJSONRPCClient(
			uris[0],
			networkID,
			chainID,
		), ws, nil
}

func (*Handler) GetBalance(
	ctx context.Context,
	cli *brpc.JSONRPCClient,
	addr codec.Address,
) (uint64, error) {
	saddr, err := codec.AddressBech32(consts.HRP, addr)
	if err != nil {
		return 0, err
	}
	balance, err := cli.Balance(ctx, saddr)
	if err != nil {
		return 0, err
	}
	if balance == 0 {
		utils.Outf("{{red}}balance:{{/}} 0 %s\n", consts.Symbol)
		utils.Outf("{{red}}please send funds to %s{{/}}\n", saddr)
		utils.Outf("{{red}}exiting...{{/}}\n")
		return 0, nil
	}
	utils.Outf(
		"{{yellow}}balance:{{/}} %s %s\n",
		utils.FormatBalance(balance, consts.Decimals),
		consts.Symbol,
	)
	return balance, nil
}

type Controller struct {
	databasePath string
}

func NewController(databasePath string) *Controller {
	return &Controller{databasePath}
}

func (c *Controller) DatabasePath() string {
	return c.databasePath
}

func (*Controller) Symbol() string {
	return consts.Symbol
}

func (*Controller) Decimals() uint8 {
	return consts.Decimals
}

func (*Controller) Address(addr codec.Address) string {
	return codec.MustAddressBech32(consts.HRP, addr)
}

func (*Controller) ParseAddress(addr string) (codec.Address, error) {
	return codec.ParseAddressBech32(consts.HRP, addr)
}
