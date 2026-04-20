// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// Package scriptlets manages the execution of Starlark scripts within the Juju
// controller.
//
// Scriptlets are user-defined Starlark programs that run as managed child
// workers. The package provides a top-level worker that uses a catacomb for
// lifecycle management and a worker.Runner to spawn, supervise, and restart
// individual scriptlet workers. A watcher subscription drives the creation of
// new scriptlet workers in response to change events.
//
// See github.com/juju/juju/internal/worker for shared worker utilities. See
// go.starlark.net for the Starlark interpreter used to execute scripts.
package scriptlets
