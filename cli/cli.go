// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"github.com/AnomalyFi/hypersdk/pebble"
	"github.com/ava-labs/avalanchego/database"
)

type Handler struct {
	c Controller

	db database.Database
}

func New(c Controller) (*Handler, error) {
	db, _, err := pebble.New(c.DatabasePath(), pebble.NewDefaultConfig())
	if err != nil {
		return nil, err
	}
	return &Handler{c, db}, nil
}
