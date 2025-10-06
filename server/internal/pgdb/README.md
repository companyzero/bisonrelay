pgdb
====

## Initial Postgres Setup

Some initial setup to create a role for authentication to the Postgres server, a
couple of tablespaces to specify where the bulk data and indexes are stored, as
well as the database itself is required.  The following are the necessary
commands:

Performance tip:

* The bulk and index tablespaces should ideally be on separate physical media

```sh
$ mkdir -p /path/to/bulk_data
$ mkdir -p /path/to/index_data
$ psql -U postgres
```

At the `psql` shell:
```sql
postgres=# CREATE ROLE brdata WITH LOGIN REPLICATION NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT CONNECTION LIMIT -1 PASSWORD 'xxxxxx';
postgres=# CREATE TABLESPACE brbulk OWNER brdata LOCATION '/path/to/bulk_data';
postgres=# CREATE TABLESPACE brindex OWNER brdata LOCATION '/path/to/index_data';
postgres=# CREATE DATABASE brdata OWNER brdata TABLESPACE brbulk;
```

It is also highly recommended to configure the Postgres server to enable SSL/TLS
if not already done.  This can be accomplished by setting `ssl = on` in the
server's `postgresql.conf` file and generating a server certificate and key.
The Postgres documentation contains instructions for using `openssl` if desired.
These instructions instead opt to use the `github.com/decred/dcrd/cmd/gencerts`
utility instead:

Notes:

* The referenced data path is the same directory that houses `postgresql.conf`
* The `-H` parameter may be specified multiple times and must be the hostnames
  or IPs that certificate is valid for
* The `-L` option makes the certificate valid for use by localhost as well

```
$ cd /path/to/postgres/data
$ gencerts -H "server hostname" -L -o "postgres" server.crt server.key
```

## Usage

```Go
func example() error {
	// Open a connection to the database.  Use `pgdb.WithHost` to specify a
	// remote host.  See the documentation for other functions that start with
	// the prefix "With" for additional configuration operations.
	ctx := context.Background()
	db, err := pgdb.Open(ctx, pgdb.WithPassphrase("xxxxxx"),
		pgdb.WithTLS("/path/to/server.crt"))
	if err != nil {
		return err
	}
	defer db.Close()

	// Store a payload at a given rendezvous point.  The rv would ordinarily
	// come from an actual derivation.
	rv := [32]byte{0x01}
	payload := []byte{0x0a, 0x0b, 0x0c}
	err = db.StorePayload(ctx, rv[:], payload, time.Now())
	if err != nil {
		return err
	}

	// Store a different payload at a different rendezvous point.
	rv2 := [32]byte{0x02}
	payload2 := []byte{0x0d, 0x0e, 0x0f}
	err = db.StorePayload(ctx, rv2[:], payload2, time.Now())
	if err != nil {
		return err
	}

	// Fetch the payload at a given rendezvous point.
	result, err := db.FetchPayload(ctx, rv[:])
	if err != nil {
		return err
	}
	if result == nil {
		fmt.Printf("no payload for rv %x\n", rv)
	} else {
		fmt.Printf("got payload len %v\n", len(result.Payload))
	}

	// Remove the payload for a given rendezvous point.
	err = db.RemovePayload(ctx, rv[:])
	if err != nil {
		return err
	}

	// All entries inserted on a given day can be expired in bulk.  The
	// caller is expected to call this on a schedule to cleanup payloads
	// that have never been retrieved and removed.
	dateToExpire := time.Now().UTC()
	numExpired, err := db.Expire(ctx, dateToExpire)
	if err != nil {
		return err
	}
	fmt.Printf("Expired %d entries for %v\n", numExpired,
		dateToExpire.Format("2006-01-02"))

	return nil
}

func main() {
	if err := example(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
```

## Simulation Tool

A simulation tool for helping test performance and correctness is available in
`cmd/pgdbsim`.  See its [README.md](./cmd/pgdbsim/README.md) for further
details.

## License

pgdb is licensed under the [copyfree](http://copyfree.org) ISC License.
