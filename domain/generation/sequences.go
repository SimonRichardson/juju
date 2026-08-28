// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package generation

import "github.com/juju/juju/domain/sequence"

const (
	// GenerationSequenceNamespace is the namespace for the generation_id
	// sequence. generation_id is a human-facing identifier preserved across
	// commits; it does not define branch or commit ordering.
	GenerationSequenceNamespace = sequence.StaticNamespace("generation")
)
