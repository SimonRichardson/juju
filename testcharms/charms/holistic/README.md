# Holistic test charm

This is a deliberately small bash charm for exercising the holistic uniter.

`charmcraft.yaml` is the project source of truth. Charmcraft generates the
charm metadata and configuration files when the charm is packed.

The `install`, `reconcile`, `start`, `stop`, and `remove` hooks are separate
executable entry points. `install`, `start`, `stop`, and `remove` express
lifecycle setup or teardown; all snapshot-derived changes, including config
and charm upgrades, dispatch `reconcile`.

The reconciler records the event and its complete state is supplied through
the hook environment. Collection values, such as `JUJU_RELATIONS`, are JSON.
