#!/bin/sh
PG_USER=postgres
PG_DATADIR=/var/lib/postgres/data

set -e

# stop postgresql.
/usr/bin/sudo -u $PG_USER /usr/local/bin/pg_ctl -D $PG_DATADIR stop

# rewind postgresql.
# https://www.postgresql.org/docs/current/app-pgrewind.html
/usr/bin/sudo -u $PG_USER /usr/local/bin/pg_rewind -D /var/lib/postgres/data --source-server="host=192.168.0.1 port=5432 user=rewind_user password=rewind_user dbname=postgres"

# set postgresql to secondary mode.
/usr/bin/sudo -u $PG_USER /usr/bin/touch $PG_DATADIR/standby.signal

# copy local postgres config changes.
/usr/bin/sudo -u $PG_USER /usr/bin/cp /var/lib/postgres/postgresql.local.conf /var/lib/postgres/data

# start postgresql.
/usr/bin/sudo -u $PG_USER /usr/local/bin/pg_ctl -D $PG_DATADIR start
