# Holistic test charm

This is a deliberately small bash charm for exercising the holistic uniter.

`charmcraft.yaml` is the project source of truth. Charmcraft generates the
charm metadata and configuration files when the charm is packed.

The `install`, `config-changed`, `reconcile`, `start`, `upgrade-charm`, `stop`,
and `remove` hooks are separate executable entry points. Each delegates to
`hooks/holistic-reconcile`, so the charm can share idempotent reconciliation
logic without losing lifecycle event identity.

The reconciler records the event and, when the runtime provides it, invokes the
`unit-snapshot` jujuc command. This keeps the test useful before and after the
Ops framework gains its native snapshot accessor.
