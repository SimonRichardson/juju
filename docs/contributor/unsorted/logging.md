(logging)=
# Logging


The intent of this documentation is to provide an overview of logging
in Juju.

## Accessing logs as a user

Please consult the available user documentation for details of how to
access Juju's logs. Specifically:

  * https://juju.is/docs/olm/juju-logs
  * juju help logging
  * juju help debug-log
  * juju-dumplogs --help

## Consolidated log infrastructure

All machine and unit agents run a "logsender" worker which sends the
agent's logs to a controller using the "logsink" API. The "logsink"
API handler writes the received logs to a "logs" collection in the
"logs" MongoDB database (via DbLogger in the state package). In a HA
model the logs are replicated between controllers using
MongoDB's usual replication mechanisms.

The `juju debug-log` command uses the "logs" API to retrieve
logs. This starts by querying the logs.logs collection and then
"tails" the replication oplog for further updates to logs.logs. The
LogTailer in the state package abstracts away the log querying and
tailing mechanics.
