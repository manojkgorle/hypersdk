// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/AnomalyFi/hypersdk/consts"
)

const (
	ReadBufferSize      = 512 * units.KiB
	WriteBufferSize     = 512 * units.KiB
	WriteWait           = 10 * time.Second
	PongWait            = 60 * time.Second
	PingPeriod          = (PongWait * 9) / 10
	MaxReadMessageSize  = consts.NetworkSizeLimit
	MaxWriteMessageSize = 16 * units.MiB
	MaxMessageWait      = 50 * time.Millisecond
	MaxPendingMessages  = 1024
)
