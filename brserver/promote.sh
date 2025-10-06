#!/bin/sh
PG_USER=postgres
PG_DATADIR=/var/lib/postgres/data

set -e

# promote postgresql.
/usr/bin/sudo -u $PG_USER /usr/local/bin/pg_ctl -D $PG_DATADIR promote
