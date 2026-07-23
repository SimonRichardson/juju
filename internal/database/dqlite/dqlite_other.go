//go:build !dqlite

// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package dqlite

import "hash/fnv"

const (
	// Enabled is false if dqlite is disabled.
	Enabled = false
)

type NodeRole int

func (NodeRole) String() string {
	return ""
}

// Dqlite node roles.
const (
	Voter   = 1
	StandBy = 2
	Spare   = 3
)

type NodeInfo struct {
	ID      uint64   `yaml:"ID"`
	Address string   `yaml:"Address"`
	Role    NodeRole `yaml:"Role"`
}

func ReconfigureMembership(string, []NodeInfo) error {
	return nil
}

// GenerateID generates a stable stand-in node ID when Dqlite is disabled.
func GenerateID(address string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(address))
	return hash.Sum64()
}
