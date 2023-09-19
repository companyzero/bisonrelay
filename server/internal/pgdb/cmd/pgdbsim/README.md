pgdbsim
=======

This is a simulation tool for helping test performance and correctness.

## Initial Postgres Setup

This tool requires the same initial setup that is detailed in the [pgdb README.md initial postgres setup section](../../README.md#initial-postgres-setup) with the exception of the database name, which is `brdatasim` by default.

Thus, after following the aforementioned setup instructions, run the following commands to create the database:

```sh
$ psql -U postgres
```

At the `psql` shell:
```sql
postgres=# CREATE DATABASE brdatasim OWNER brdata TABLESPACE brbulk;
```

## Running a Simulation

Run the following command and enter the password when prompted:

```sh
$ go build && ./pgdbsim
```

## Configuring the Simulation

Several command line flags are available to modify the behavior of the
simulation and configure the database connection parameters.  Use `-h` to see a
full list.  The following are the ones most likely to be used:

* `-notls` disable TLS
* `-days` The number of days to simulate (default 7)
* `-maxchunksize` maximum chunk size for payloads (default 1048576)
* `-minchunksize` minimum chunk size for payloads (default 256)
* `-totalbytes` total number of bytes to insert during simulation (default
  2147483648)

## License

pgdbsim is licensed under the [copyfree](http://copyfree.org) ISC License.
