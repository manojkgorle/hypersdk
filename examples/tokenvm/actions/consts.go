// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

// Note: Registry will error during initialization if a duplicate ID is assigned. We explicitly assign IDs to avoid accidental remapping.
const (
	burnAssetID      uint8 = 0
	closeOrderID     uint8 = 1
	createAssetID    uint8 = 2
	exportAssetID    uint8 = 3
	importAssetID    uint8 = 4
	createOrderID    uint8 = 5
	fillOrderID      uint8 = 6
	mintAssetID      uint8 = 7
	transferID       uint8 = 8
	deployContractID uint8 = 9
	txID             uint8 = 10
	txID2            uint8 = 11
)

const (
	// TODO: tune this
	BurnComputeUnits        = 2
	CloseOrderComputeUnits  = 5
	CreateAssetComputeUnits = 10
	ExportAssetComputeUnits = 10
	ImportAssetComputeUnits = 10
	CreateOrderComputeUnits = 5
	NoFillOrderComputeUnits = 5
	FillOrderComputeUnits   = 15
	MintAssetComputeUnits   = 2
	TransferComputeUnits    = 1
	TransactMaxComputeUnits = 10_000_000
	TempComputeUnits        = 10
	MaxSymbolSize           = 8
	MaxMemoSize             = 256
	MaxMetadataSize         = 256
	MaxDecimals             = 9
)

const (
	// gas-metering
	BaseComputeUnits            = 30000
	StoreUintComputeUnits       = 3000
	StoreIntComputeUnits        = 3005
	StoreFloatComuteUnits       = 3005
	BaseStoreStringComputeUnits = 6000
	StoreStringIncComputeUnits  = 300
	BaseStoreBytesComputeUnits  = 6500
	StoreBytesIncComputeUnits   = 350
	BaseArrayComputeUnits       = 1000
	ArrayAppendComputeUnits     = 500
	ArrayPopComputeUnits        = 600
	ArrayInsertComputeUnits     = 800
	ArrayReplaceComputeUnits    = 1000
	ArrayDeleteComputeUnits     = 100
)

const (
	GetUintComputeUints      = 500
	GetIntComputeUnits       = 500
	GetFloatComputeUnits     = 500
	GetStringComputeUnits    = 600
	BaseGetArrayComputeUnits = 400
	BaseGetBytesComputeUnits = 400
	GetBytesComputeUnits     = 50
	GetArrayIncComputeUnits  = 50
)

const (
	BaseCALLUnits         = 10_000
	BaseDELEGATECALLUnits = 12_000
)

const (
	FunctionNotFoundComputeUnits     = 30_000
	WrongPointerOrLengthComputeUnits = 50_000
)
