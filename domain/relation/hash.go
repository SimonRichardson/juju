// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package relation

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/juju/juju/internal/errors"
)

// Setting is a key/value pair representing a relation setting.
type Setting struct {
	Key   string
	Value string
}

// HashSettings returns a sha256 hash of the provided settings.
func HashSettings(settings []Setting) (string, error) {
	h := sha256.New()

	// Ensure we have a stable order for the keys.
	sort.Slice(settings, func(i, j int) bool {
		return settings[i].Key < settings[j].Key
	})

	for _, s := range settings {
		if _, err := h.Write([]byte(s.Key + " " + s.Value + " ")); err != nil {
			return "", errors.Errorf("writing relation setting: %w", err)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
