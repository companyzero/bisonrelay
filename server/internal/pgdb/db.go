package pgdb

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/server/serverdb"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"
)

const (
	// DefaultHost is the default host that serves the backing database.
	DefaultHost = "127.0.0.1"

	// DefaultPort is the default port for the host that serves the backing
	// database.
	DefaultPort = "5432"

	// DefaultDBName is the default name for the backing database.
	DefaultDBName = "brdata"

	// DefaultRoleName is the default name for the role used to access the
	// database.
	DefaultRoleName = "brdata"

	// DefaultIndexTablespaceName is the default name for the tablespace that
	// is used to store the indexes.
	DefaultIndexTablespaceName = "brindex"

	// DefaultBulkDataTablespaceName is the default name for the tablespace that
	// is used to store the bulk payload data.
	DefaultBulkDataTablespaceName = "brbulk"
)

const (
	// currentDBVersion indicates the current database version.
	currentDBVersion = 3

	// pgDateFormat is the format string to use when specifying the date ranges
	// for partitions in the format Postgres understands such that they refer to
	// specific days.
	pgDateFormat = "2006-01-02"
)

// databaseInfo houses information about the state of the database such as its
// version and the time it was created.
type databaseInfo struct {
	version uint32
	created time.Time
	updated time.Time
}

// DB provides access to the backend database for storing, retrieving, and
// removing payloads associated with rendezvous points along with additional
// support for bulk expiration by date.
type DB struct {
	// The following values allow configuration of the database and are only set
	// when the instance is created, so no mutex is needed to protect concurrent
	// access.
	//
	// dbName is the name of the name of the postgres database that is used to
	// house all of the data.
	//
	// roleName is the name of the role that is used to perform all database
	// operations.
	//
	// indexTablespace is the name of the tablespace that is used to store the
	// index information.
	//
	// bulkDataTablespace is the name of the tablespace that is used to store
	// the raw payload data.
	dbName             string
	roleName           string
	indexTablespace    string
	bulkDataTablespace string

	// initMtx protect concurrent access during the database initialization and
	// also protects the following fields:
	//
	// dbInfo houses information about the database such as its version and when
	// it was created.
	initMtx sync.Mutex
	dbInfo  *databaseInfo

	// db houses the handle to the underlying Postgres database.
	db *pgxpool.Pool

	// partitionMtx protects the following fields:
	//
	// dataPartitions houses all of the data partitions (tables) that are known
	// to exist in the database.  It is used to determine when a partition for a
	// given day needs to be created.
	//
	// paidSubsPartitions houses all the known partitions for the paid
	// subscriptions table.
	//
	// redeemedPushesPartitions houses all the known partitions for the
	// redeemed push payments table.
	partitionMtx             sync.Mutex
	dataPartitions           map[string]struct{}
	paidSubsPartitions       map[string]struct{}
	redeemedPushesPartitions map[string]struct{}
}

func sqlTxROInternal(ctx context.Context, conn *pgx.Conn, f func(tx pgx.Tx) error) error {
	return sqlTxWithOptions(ctx, conn, pgx.TxOptions{AccessMode: pgx.ReadOnly}, f)
}

// sqlTx runs the provided function inside of SQL transaction and will either
// rollback the transaction and return the error when a non-nil error is
// returned from the provided function or commit the transaction when a nil
// error is returned the provided function.
func (db *DB) sqlTx(ctx context.Context, f func(tx pgx.Tx) error) (err error) {
	conn, err := db.db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	return sqlTxInternal(ctx, conn.Conn(), f)
}

func sqlTxInternal(ctx context.Context, conn *pgx.Conn, f func(tx pgx.Tx) error) error {
	return sqlTxWithOptions(ctx, conn, pgx.TxOptions{}, f)
}

func sqlTxWithOptions(ctx context.Context, conn *pgx.Conn, txOptions pgx.TxOptions, f func(tx pgx.Tx) error) (err error) {
	tx, err := conn.BeginTx(ctx, txOptions)
	if err != nil {
		str := fmt.Sprintf("unable to start transaction: %v", err)
		return contextError(ErrBeginTx, str, err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}
		err = tx.Commit(ctx)
		if err != nil {
			str := fmt.Sprintf("unable to commit transaction: %v", err)
			err = contextError(ErrCommitTx, str, err)
		}
	}()
	return f(tx)
}

// checkRoleExists returns an error if the given role does not exist.
func checkRoleExists(ctx context.Context, tx pgx.Tx, roleName string) error {
	const query = "SELECT COUNT(*) FROM pg_roles WHERE rolname = $1;"
	var count uint64
	row := tx.QueryRow(ctx, query, roleName)
	if err := row.Scan(&count); err != nil {
		str := fmt.Sprintf("unable to query role: %v", err)
		return contextError(ErrQueryFailed, str, err)
	}
	if count != 1 {
		help := fmt.Sprintf("SQL to resolve: CREATE ROLE %s WITH LOGIN "+
			"NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION "+
			"CONNECTION LIMIT -1 PASSWORD 'xxxxxx';\nNOTE: Creating a role "+
			"typically requires admin permissions", pq.QuoteIdentifier(roleName))
		str := fmt.Sprintf("invalid db config: role %q does not exist -- %s",
			roleName, help)
		return contextError(ErrMissingRole, str, nil)
	}
	return nil
}

// checkTablespaceExists returns an error if the given tablespace does not
// exist.
func (db *DB) checkTablespaceExists(ctx context.Context, tx pgx.Tx, tablespace string) error {
	const query = "SELECT COUNT(*) FROM pg_tablespace WHERE spcname = $1;"
	var count uint64
	row := tx.QueryRow(ctx, query, tablespace)
	if err := row.Scan(&count); err != nil {
		str := fmt.Sprintf("unable to query tablespace: %v", err)
		return contextError(ErrQueryFailed, str, err)
	}
	if count != 1 {
		const path = "/path/to/storage/"
		help := fmt.Sprintf("SQL to resolve: CREATE TABLESPACE %s OWNER %s "+
			"LOCATION %s;\nNOTE: Creating a tablespace typically requires "+
			"admin permissions", pq.QuoteIdentifier(tablespace),
			pq.QuoteIdentifier(db.roleName), pq.QuoteLiteral(path))
		str := fmt.Sprintf("invalid db config: tablespace %q does not exist -- %s",
			tablespace, help)
		return contextError(ErrMissingTablespace, str, nil)
	}
	return nil
}

// checkDatabaseExists returns an error if the given database does not exist.
func (db *DB) checkDatabaseExists(ctx context.Context, tx pgx.Tx, dbName string) error {
	const query = "SELECT COUNT(*) FROM pg_database WHERE datname = $1;"
	var count uint64
	row := tx.QueryRow(ctx, query, dbName)
	if err := row.Scan(&count); err != nil {
		str := fmt.Sprintf("unable to query db: %v", err)
		return contextError(ErrQueryFailed, str, err)
	}
	if count != 1 {
		help := fmt.Sprintf("SQL to resolve: CREATE DATABASE %s OWNER %s "+
			"TABLESPACE %s;\nNOTE: Creating a database typically requires "+
			"admin permissions", pq.QuoteIdentifier(dbName),
			pq.QuoteIdentifier(db.roleName),
			pq.QuoteIdentifier(db.bulkDataTablespace))
		str := fmt.Sprintf("invalid db config: database %q does not exist -- %s",
			dbName, help)
		return contextError(ErrMissingDatabase, str, nil)
	}
	return nil
}

// checkDatabaseInitialized returns an error if the current db connection is
// not properly initialized with the necessary tablespaces and database.
func (db *DB) checkDatabaseInitialized(ctx context.Context, tx pgx.Tx) error {
	err := checkRoleExists(ctx, tx, db.roleName)
	if err != nil {
		return err
	}
	err = db.checkTablespaceExists(ctx, tx, db.bulkDataTablespace)
	if err != nil {
		return err
	}
	err = db.checkTablespaceExists(ctx, tx, db.indexTablespace)
	if err != nil {
		return err
	}
	return db.checkDatabaseExists(ctx, tx, db.dbName)
}

// createDbInfoTableQuery returns a SQL query that creates the database info
// table (with no rows) if it does not already exist.
func (db *DB) createDbInfoTableQuery() string {
	const query = "CREATE TABLE IF NOT EXISTS db_info (" +
		"	id INTEGER PRIMARY KEY NOT NULL DEFAULT (1) CHECK(id = 1)," +
		"	version INTEGER NOT NULL CHECK (version > 0)," +
		"	created TIMESTAMP NOT NULL DEFAULT (NOW())," +
		"	updated TIMESTAMP NOT NULL DEFAULT (NOW())" +
		") TABLESPACE %s;"
	tablespace := pq.QuoteIdentifier(db.bulkDataTablespace)
	return fmt.Sprintf(query, tablespace)
}

// maybeLoadDatabaseInfo attempts to information about the state of the database
// such as its version and the time it was created.  It returns nil for both the
// database info and the error when the information does not exist yet.
func maybeLoadDatabaseInfo(ctx context.Context, tx pgx.Tx) (*databaseInfo, error) {
	var dbInfo databaseInfo
	const query = "SELECT version, created, updated FROM db_info WHERE id = 1;"
	row := tx.QueryRow(ctx, query)
	err := row.Scan(&dbInfo.version, &dbInfo.created, &dbInfo.updated)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		str := fmt.Sprintf("unable to query database info: %v", err)
		return nil, contextError(ErrQueryFailed, str, err)
	}

	return &dbInfo, nil
}

// updateDatabaseInfo either inserts or updates the only row allowed to be in
// the database info table with the provided values.  Note that updated field of
// the passed database info will be updated to the current time (in UTC).
func updateDatabaseInfo(ctx context.Context, tx pgx.Tx, dbInfo *databaseInfo) error {
	dbInfo.updated = time.Now().UTC()

	const query = "INSERT INTO db_info (version, created, updated) VALUES " +
		"($1, $2, $3) " +
		"ON CONFLICT (id) " +
		"DO UPDATE SET (version, updated) = ($1, $3);"
	_, err := tx.Exec(ctx, query, dbInfo.version, dbInfo.created, dbInfo.updated)
	if err != nil {
		str := fmt.Sprintf("unable to insert database info: %v", err)
		return contextError(ErrQueryFailed, str, err)
	}
	return nil
}

// createDataTableQuery returns a SQL query that creates the main virtual data
// table that is partioned by range if it does not already exist.
func (db *DB) createDataTableQuery() string {
	const query = "CREATE TABLE IF NOT EXISTS data (" +
		"	rendezvous_point TEXT NOT NULL," +
		"	payload BYTEA NOT NULL," +
		"	insert_time DATE NOT NULL," +
		"       insert_ts TIMESTAMP NOT NULL DEFAULT current_timestamp," +
		"	UNIQUE(rendezvous_point, insert_time) USING INDEX TABLESPACE %s" +
		") PARTITION BY RANGE (insert_time);"
	tablespace := pq.QuoteIdentifier(db.indexTablespace)
	return fmt.Sprintf(query, tablespace)
}

// createPaidSubsTableQuery returns a SQL query that creates the virtual data
// table that tracks paid rendezvous points and that is partioned by range if it
// does not already exist.
func (db *DB) createPaidSubsTableQuery() string {
	const query = "CREATE TABLE IF NOT EXISTS paid_subs (" +
		"	rendezvous_point TEXT NOT NULL," +
		"	insert_time DATE NOT NULL," +
		"       insert_ts timestamp not null DEFAULT current_timestamp," +
		"	UNIQUE(rendezvous_point, insert_time) USING INDEX TABLESPACE %s" +
		") PARTITION BY RANGE (insert_time);"
	tablespace := pq.QuoteIdentifier(db.indexTablespace)
	return fmt.Sprintf(query, tablespace)
}

// createRedeemedPushPaymentsQuery returns a SQL query that creates the virtual
// data table that tracks payment hashes for pushed messages that were redeemed
// and that is partioned by range if it does not already exist.
func (db *DB) createRedeemedPushPaymentsQuery() string {
	const query = "CREATE TABLE IF NOT EXISTS redeemed_push_payments (" +
		"	payment_id TEXT NOT NULL," +
		"	insert_time DATE NOT NULL," +
		"	UNIQUE(payment_id, insert_time) USING INDEX TABLESPACE %s" +
		") PARTITION BY RANGE (insert_time);"
	tablespace := pq.QuoteIdentifier(db.indexTablespace)
	return fmt.Sprintf(query, tablespace)
}

// procedureExists returns whether or not the provided stored procedure exists.
func procedureExists(ctx context.Context, tx pgx.Tx, procName string) (bool, error) {
	//	--SELECT * FROM information_schema.routines WHERE routine_name = 'global_data_rv_unique';
	const query = "SELECT COUNT(*) FROM information_schema.routines WHERE " +
		"routine_name = $1;"
	var count uint64
	row := tx.QueryRow(ctx, query, procName)
	if err := row.Scan(&count); err != nil {
		str := fmt.Sprintf("unable to query db routines: %v", err)
		return false, contextError(ErrQueryFailed, str, err)
	}
	return count > 0, nil
}

// createUniqueRVTriggerProcQuery returns a SQL query that creates a proc that
// returns a trigger to enforce a global uniqueness constraint on the rendezvous
// point across all partitions if it does not already exist.
func createUniqueRVTriggerProcQuery(procName string) string {
	const query = "CREATE OR REPLACE FUNCTION %s()" +
		" RETURNS trigger" +
		" LANGUAGE plpgsql " +
		"AS $$ " +
		"BEGIN" +
		" PERFORM pg_advisory_xact_lock(hashtext(NEW.rendezvous_point));" +
		" IF COUNT(1) > 1 FROM data WHERE rendezvous_point = NEW.rendezvous_point THEN" +
		"  RAISE EXCEPTION 'duplicate key value violates unique constraint \"%%\" ON \"%%\"'," +
		"   TG_NAME, TG_TABLE_NAME" +
		"   USING DETAIL = format('Key (rendezvous_point)=(%%s) already exists.', NEW.rendezvous_point)," +
		"     ERRCODE = 23505, CONSTRAINT = 'global_unique_data_rendezvous';" +
		" END IF;" +
		" RETURN NULL;" +
		"END " +
		"$$;"
	return fmt.Sprintf(query, procName)
}

// triggerExists returns whether or not the provided trigger exists.
func triggerExists(ctx context.Context, tx pgx.Tx, procName string) (bool, error) {
	const query = "SELECT COUNT(*) FROM information_schema.triggers WHERE " +
		"trigger_name = $1;"
	var count uint64
	row := tx.QueryRow(ctx, query, procName)
	if err := row.Scan(&count); err != nil {
		str := fmt.Sprintf("unable to query db triggers: %v", err)
		return false, contextError(ErrQueryFailed, str, err)
	}
	return count > 0, nil
}

// createUniqueRVConstraintQuery returns a SQL query that creates a constraint
// trigger that enforces a global uniqueness constraint on the rendezvous point
// across all partitions.
func createUniqueRVConstraintQuery(triggerName, procName string) string {
	const query = "CREATE CONSTRAINT TRIGGER %s AFTER INSERT OR UPDATE ON data " +
		"DEFERRABLE INITIALLY IMMEDIATE FOR EACH ROW EXECUTE PROCEDURE %s()"
	return fmt.Sprintf(query, triggerName, procName)
}

// discoverExistingPartitions queries the database to find all existing
// partitions in for the given virtual table and returns the found map of
// partitions.
func (db *DB) discoverExistingPartitions(ctx context.Context, tx pgx.Tx, baseTableName string) (map[string]struct{}, error) {
	// Ensure exclusive access to the data table when modifying or reading
	// partitions from the database.
	queryLock := fmt.Sprintf("LOCK TABLE %s", pq.QuoteIdentifier(baseTableName))
	_, err := tx.Exec(ctx, queryLock)
	if err != nil {
		str := fmt.Sprintf("unable to lock %s table: %v", baseTableName, err)
		return nil, contextError(ErrQueryFailed, str, err)
	}

	query := fmt.Sprintf("SELECT tablename FROM pg_tables WHERE tablename LIKE '%s_%%';",
		baseTableName)
	rows, err := tx.Query(ctx, query)
	if err != nil {
		str := fmt.Sprintf("unable to query tables for %s partition: %v",
			baseTableName, err)
		return nil, contextError(ErrQueryFailed, str, err)
	}
	defer rows.Close()
	parts := make(map[string]struct{})
	for rows.Next() {
		var tableName string
		if err = rows.Scan(&tableName); err != nil {
			str := fmt.Sprintf("unable to scan %s partition table name: %v",
				baseTableName, err)
			return nil, contextError(ErrQueryFailed, str, err)
		}
		parts[tableName] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		str := fmt.Sprintf("unable to scan tables for %s partition: %v",
			baseTableName, err)
		return nil, contextError(ErrQueryFailed, str, err)
	}

	return parts, nil
}

// discoverExistingDataPartitions finds all existing partitions of the the main
// data virtual table.
func (db *DB) discoverExistingDataPartitions(ctx context.Context, tx pgx.Tx) error {
	db.partitionMtx.Lock()
	defer db.partitionMtx.Unlock()

	parts, err := db.discoverExistingPartitions(ctx, tx, "data")
	if err != nil {
		return err
	}
	db.dataPartitions = parts
	return nil
}

// discoverExistingPaidSubsPartitions finds all existing partitions of the paid
// subscriptions virtual table.
func (db *DB) discoverExistingPaidSubsPartitions(ctx context.Context, tx pgx.Tx) error {
	db.partitionMtx.Lock()
	defer db.partitionMtx.Unlock()

	parts, err := db.discoverExistingPartitions(ctx, tx, "paid_subs")
	if err != nil {
		return err
	}
	db.paidSubsPartitions = parts
	return nil
}

// discoverExistingRedeemedPushPayments finds all existing partitions of the
// the redeemed push payments table.
func (db *DB) discoverExistingRedeemedPushPayments(ctx context.Context, tx pgx.Tx) error {
	db.partitionMtx.Lock()
	defer db.partitionMtx.Unlock()

	parts, err := db.discoverExistingPartitions(ctx, tx, "redeemed_push_payments")
	if err != nil {
		return err
	}
	db.redeemedPushesPartitions = parts
	return nil
}

// upgradeDB upgrades old database versions to the newest version by applying
// all possible upgrades iteratively.
//
// NOTE: The passed database info will be updated with the latest versions.
func upgradeDB(ctx context.Context, tx pgx.Tx, dbInfo *databaseInfo, indexTablespace string) error {
	if dbInfo.version == 1 {
		if err := upgradeDBToV2(ctx, tx, dbInfo); err != nil {
			return err
		}
		dbInfo.version = 2
		if err := updateDatabaseInfo(ctx, tx, dbInfo); err != nil {
			return err
		}
	}

	if dbInfo.version == 2 {
		if err := upgradeDBToV3(ctx, tx, dbInfo, indexTablespace); err != nil {
			return err
		}
		dbInfo.version = 3
		if err := updateDatabaseInfo(ctx, tx, dbInfo); err != nil {
			return err
		}
	}

	return nil
}

// initDB initializes the database to include loading the database version and
// potentially creating any tables, triggers, and constraints that are needed
// for proper operation.
//
// This function MUST be called with the init mutex held (for writes).
func (db *DB) initDB(ctx context.Context, tx pgx.Tx) error {
	// Ensure exclusive access during initialization.
	const dbInfoAdvisoryLockID = 1000
	const query = "SELECT pg_advisory_xact_lock(%d);"
	_, err := tx.Exec(ctx, fmt.Sprintf(query, dbInfoAdvisoryLockID))
	if err != nil {
		str := fmt.Sprintf("unable to obtain init lock: %v", err)
		return contextError(ErrQueryFailed, str, err)
	}

	// Create the database info table if it does not already exist.
	_, err = tx.Exec(ctx, db.createDbInfoTableQuery())
	if err != nil {
		str := fmt.Sprintf("unable to create database info table: %v", err)
		return contextError(ErrQueryFailed, str, err)
	}

	// Attempt to load database info to determine the state of the database.
	db.dbInfo, err = maybeLoadDatabaseInfo(ctx, tx)
	if err != nil {
		return err
	}

	// Create any tables, triggers, and constraints that are needed for proper
	// operation when the database has not yet been initialized.
	if db.dbInfo == nil {
		// Populate the database version info.
		now := time.Now().UTC()
		db.dbInfo = &databaseInfo{
			version: currentDBVersion,
			created: now,
		}
		if err := updateDatabaseInfo(ctx, tx, db.dbInfo); err != nil {
			return err
		}

		// Create the main virtual partitioned data table if needed.
		_, err := tx.Exec(ctx, db.createDataTableQuery())
		if err != nil {
			str := fmt.Sprintf("unable to create data table: %v", err)
			return contextError(ErrQueryFailed, str, err)
		}

		// Create a trigger procedure that is used to enforce a global
		// uniqueness constraint on the rendezvous point across all partitions
		// if it does not already exist.
		const procName = "global_data_rv_unique"
		exists, err := procedureExists(ctx, tx, procName)
		if err != nil {
			return err
		}
		if !exists {
			query := createUniqueRVTriggerProcQuery(procName)
			if _, err := tx.Exec(ctx, query); err != nil {
				str := fmt.Sprintf("unable to create stored procedure: %v", err)
				return contextError(ErrQueryFailed, str, err)
			}
		}

		// Create a constraint trigger on the main virtual partitioned data
		// table that will be inherited by all partitions unless it already
		// exists.
		const triggerName = "partition_data_rv_unique"
		exists, err = triggerExists(ctx, tx, triggerName)
		if err != nil {
			return err
		}
		if !exists {
			query := createUniqueRVConstraintQuery(triggerName, procName)
			if _, err := tx.Exec(ctx, query); err != nil {
				str := fmt.Sprintf("unable to create constraint: %v", err)
				return contextError(ErrQueryFailed, str, err)
			}
		}

		// Create the main virtual partitioned paid subscriptions table if needed.
		_, err = tx.Exec(ctx, db.createPaidSubsTableQuery())
		if err != nil {
			str := fmt.Sprintf("unable to create paid subscriptions table: %v", err)
			return contextError(ErrQueryFailed, str, err)
		}

		// Create the virtual partitioned redeemed push payments table if needed.
		_, err = tx.Exec(ctx, db.createRedeemedPushPaymentsQuery())
		if err != nil {
			str := fmt.Sprintf("unable to create redeemed push payments table: %v", err)
			return contextError(ErrQueryFailed, str, err)
		}

	}

	if db.dbInfo.version > currentDBVersion {
		str := fmt.Sprintf("the current database is no longer compatible with "+
			"this version of the software (%d > %d)", db.dbInfo.version,
			currentDBVersion)
		return contextError(ErrOldDatabase, str, err)
	}

	// Upgrade the database if needed.
	if err := upgradeDB(ctx, tx, db.dbInfo, db.indexTablespace); err != nil {
		return err
	}

	// Discover the existing data partitions and populate the internal map used
	// for partition maintenance accordingly.
	if err := db.discoverExistingDataPartitions(ctx, tx); err != nil {
		return err
	}
	if err := db.discoverExistingPaidSubsPartitions(ctx, tx); err != nil {
		return err
	}
	if err := db.discoverExistingRedeemedPushPayments(ctx, tx); err != nil {
		return err
	}
	return nil
}

// tableExists returns whether or not the provided table exists.
func tableExists(ctx context.Context, tx pgx.Tx, tableName string) (bool, error) {
	const query = "SELECT COUNT(*) FROM information_schema.tables WHERE " +
		"table_name = $1;"
	var count uint64
	row := tx.QueryRow(ctx, query, tableName)
	if err := row.Scan(&count); err != nil {
		str := fmt.Sprintf("unable to query table names: %v", err)
		return false, contextError(ErrQueryFailed, str, err)
	}
	return count > 0, nil
}

// checkBulkTablespace returns an error if the given table is not configured
// with the expected data tablespace.
func (db *DB) checkBulkTablespace(ctx context.Context, tx pgx.Tx, tableName string) error {
	const query = "SELECT tablespace FROM pg_tables WHERE tablename = $1;"
	var tablespace pgtype.Text
	row := tx.QueryRow(ctx, query, tableName)
	if err := row.Scan(&tablespace); err != nil {
		str := fmt.Sprintf("unable to query table %q tablespace: %v", tableName,
			err)
		return contextError(ErrQueryFailed, str, err)
	}

	// Lookup the default data tablespace for the database when the tablespace
	// for the table is NULL.
	if !tablespace.Valid {
		const query = "SELECT t.spcname FROM pg_database d JOIN " +
			"pg_tablespace t ON d.dattablespace = t.oid WHERE d.datname = $1;"
		row := tx.QueryRow(ctx, query, db.dbName)
		if err := row.Scan(&tablespace); err != nil {
			str := fmt.Sprintf("unable to query database %q tablespace: %v",
				db.dbName, err)
			return contextError(ErrQueryFailed, str, err)
		}
	}

	if tablespace.String != db.bulkDataTablespace {
		str := fmt.Sprintf("invalid db config: table %q has tablespace %q "+
			"instead of the expected %q", tableName, tablespace.String,
			db.bulkDataTablespace)
		return contextError(ErrBadDataTablespace, str, nil)
	}

	return nil
}

// checkIndexTablespace returns an error if the given index for the given table
// is not configured with the expected index tablespace.
func (db *DB) checkIndexTablespace(ctx context.Context, tx pgx.Tx, tableName, indexName string) error {
	const query = "SELECT tablespace FROM pg_indexes WHERE tablename = $1 " +
		"AND indexname = $2;"
	var tablespace string
	row := tx.QueryRow(ctx, query, tableName, indexName)
	if err := row.Scan(&tablespace); err != nil {
		str := fmt.Sprintf("unable to query table %q index tablespace: %v",
			tableName, err)
		return contextError(ErrQueryFailed, str, err)
	}
	if tablespace != db.indexTablespace {
		str := fmt.Sprintf("invalid db config: table %q has index tablespace "+
			"%q instead of the expected %q", tableName, tablespace,
			db.indexTablespace)
		return contextError(ErrBadIndexTablespace, str, nil)
	}
	return nil
}

// checkDatabaseSetting returns an error if the given setting name is not set to
// the provided expected value.
func checkDatabaseSetting(ctx context.Context, tx pgx.Tx, name, expected string) error {
	const query = "SELECT setting FROM pg_settings WHERE name = $1;"
	var setting string
	row := tx.QueryRow(ctx, query, name)
	if err := row.Scan(&setting); err != nil {
		str := fmt.Sprintf("unable to query db setting %q: %v", name, err)
		return contextError(ErrQueryFailed, str, err)
	}
	if setting != expected {
		str := fmt.Sprintf("invalid db config: The %q config parameter in "+
			"postgresql.conf is set to %q instead of the expected %q", name,
			setting, expected)
		return contextError(ErrBadSetting, str, nil)
	}
	return nil
}

// checkDatabaseSanity return an error if required database settings are not
// configured as needed for necessary support.
func (db *DB) checkDatabaseSanity(ctx context.Context, tx pgx.Tx) error {
	// Ensure the partition pruning config parameter is enabled.
	const pruningName = "enable_partition_pruning"
	if err := checkDatabaseSetting(ctx, tx, pruningName, "on"); err != nil {
		return err
	}

	// Ensure the database info table is configured with the expected data
	// tablespace.
	const dbInfoTableName = "db_info"
	if err := db.checkBulkTablespace(ctx, tx, dbInfoTableName); err != nil {
		return err
	}

	// Ensure the virtual partitioned tables exist.
	tables := []string{"data", "paid_subs", "redeemed_push_payments"}
	for _, tableName := range tables {
		// Ensure the main virtual partitioned data table exists.
		exists, err := tableExists(ctx, tx, tableName)
		if err != nil {
			return err
		}
		if !exists {
			str := fmt.Sprintf("invalid db config: table %q does not exist",
				tableName)
			return contextError(ErrMissingTable, str, nil)
		}

		// Ensure the virtual partitioned data table is configured with the expected
		// data tablespace.
		if err := db.checkBulkTablespace(ctx, tx, tableName); err != nil {
			return err
		}
	}

	// Ensure the data partitions are all configured with the expected data and
	// index tablespaces.
	db.partitionMtx.Lock()
	checkPartitions := []map[string]struct{}{db.dataPartitions, db.paidSubsPartitions,
		db.redeemedPushesPartitions}
	for ip, partitions := range checkPartitions {
		for tableName := range partitions {
			if err := db.checkBulkTablespace(ctx, tx, tableName); err != nil {
				return err
			}

			// Determine the correct index name expected for the given
			// table.
			var idxName string
			if ip == 2 {
				idxName = fmt.Sprintf("%s_payment_id_insert_time_key", tableName)
			} else {
				idxName = fmt.Sprintf("%s_rendezvous_point_insert_time_key", tableName)
			}

			if err := db.checkIndexTablespace(ctx, tx, tableName, idxName); err != nil {
				return err
			}
		}
	}
	db.partitionMtx.Unlock()

	// Ensure the trigger procedure that is used to enforce a global uniqueness
	// constraint on the rendezvous point across all partitions exists.
	const procName = "global_data_rv_unique"
	exists, err := procedureExists(ctx, tx, procName)
	if err != nil {
		return err
	}
	if !exists {
		str := fmt.Sprintf("invalid db config: trigger proc %q does not exist",
			procName)
		return contextError(ErrMissingProc, str, nil)
	}

	// Ensure the constraint trigger on the main virtual partitioned data table
	// that is inherited by all partitions exists.
	const triggerName = "partition_data_rv_unique"
	exists, err = triggerExists(ctx, tx, triggerName)
	if err != nil {
		return err
	}
	if !exists {
		str := fmt.Sprintf("invalid db config: trigger %q does not exist",
			triggerName)
		return contextError(ErrMissingTrigger, str, nil)
	}

	return nil
}

// partitionTableName returns the name of the data table to use for a partition
// based on the provided date.
func partitionTableName(baseTableName string, date time.Time) string {
	const tableDateFormat = "20060102"
	return fmt.Sprintf("%s_%s", baseTableName, date.Format(tableDateFormat))
}

// maybeCreatePartition creates a concrete data parition for the day associated
// with the provided date if it does not already exist.
//
// This function MUST be called with the partition mutex held (for writes).
func (db *DB) maybeCreatePartition(ctx context.Context, baseTableName string, date time.Time) error {

	partitionName := partitionTableName(baseTableName, date)
	baseTableName = pq.QuoteIdentifier(baseTableName)

	return db.sqlTx(ctx, func(tx pgx.Tx) error {
		// Ensure exclusive access to the data table when modifying or reading
		// partitions from the database.
		queryLock := fmt.Sprintf("LOCK TABLE %s", baseTableName)
		_, err := tx.Exec(ctx, queryLock)
		if err != nil {
			str := fmt.Sprintf("unable to lock %s table: %v", baseTableName, err)
			return contextError(ErrQueryFailed, str, err)
		}

		// SQL query that creates a table that is an individual concrete
		// partition for a range that is suitable for the provided date.
		const createQuery = "CREATE TABLE IF NOT EXISTS %s PARTITION OF %s " +
			"FOR VALUES FROM (%s) TO (%s) TABLESPACE %s;"
		tableName := pq.QuoteIdentifier(partitionName)
		fromDate := pq.QuoteLiteral(date.Format(pgDateFormat))
		toDate := pq.QuoteLiteral(date.Add(time.Hour * 24).Format(pgDateFormat))
		tablespace := pq.QuoteIdentifier(db.bulkDataTablespace)
		query := fmt.Sprintf(createQuery, tableName, baseTableName, fromDate, toDate, tablespace)

		_, err = tx.Exec(ctx, query)
		if err != nil {
			str := fmt.Sprintf("unable to create data partition: %v", err)
			return contextError(ErrQueryFailed, str, err)
		}

		// Track the partition so there is no attempt to create it again.
		return nil
	})
}

// maybeCreatePartitions creates the concrete partitions for the day associated
// with the provided date if it does not already exist for both data and paid
// subscriptions tables.
//
// This function MUST be called with the partition mutex held (for writes).
func (db *DB) maybeCreatePartitions(ctx context.Context, date time.Time) error {
	dataPartitionName := partitionTableName("data", date)
	if _, exists := db.dataPartitions[dataPartitionName]; !exists {
		err := db.maybeCreatePartition(ctx, "data", date)
		if err != nil {
			return err
		}
		db.dataPartitions[dataPartitionName] = struct{}{}
	}

	paidSubsPartitionName := partitionTableName("paid_subs", date)
	if _, exists := db.paidSubsPartitions[paidSubsPartitionName]; !exists {
		err := db.maybeCreatePartition(ctx, "paid_subs", date)
		if err != nil {
			return err
		}
		db.paidSubsPartitions[paidSubsPartitionName] = struct{}{}
	}

	paidRedeemedPushPayPartitionName := partitionTableName("redeemed_push_payments", date)
	if _, exists := db.redeemedPushesPartitions[paidRedeemedPushPayPartitionName]; !exists {
		err := db.maybeCreatePartition(ctx, "redeemed_push_payments", date)
		if err != nil {
			return err
		}
		db.redeemedPushesPartitions[paidRedeemedPushPayPartitionName] = struct{}{}
	}

	return nil
}

// CreatePartition creates a concrete data parition for the day associated with
// the provided date if it does not already exist.  The provided date will be
// converted to UTC if needed.
//
// Note that it is typically not necessary to call this manually since the
// partitions are managed internally by the code that handles storage and
// expiration.
//
// This is primarily provided in order to expose more flexibility for testing
// purposes.
func (db *DB) CreatePartition(ctx context.Context, date time.Time) error {
	ctx, task := trace.NewTask(ctx, "createPartition")
	defer task.End()

	date = date.UTC()

	db.partitionMtx.Lock()
	defer db.partitionMtx.Unlock()

	return db.maybeCreatePartitions(ctx, date)
}

// IsMaster returns if the server is master (handles writes).
func (db *DB) IsMaster(ctx context.Context) (bool, error) {
	conn, err := db.db.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Release()

	return isMaster(ctx, conn.Conn())
}

func isMaster(ctx context.Context, conn *pgx.Conn) (bool, error) {
	ctx, task := trace.NewTask(ctx, "db.isMaster")
	defer task.End()

	var isRecovering bool
	err := conn.QueryRow(ctx, "SELECT pg_is_in_recovery();").Scan(&isRecovering)
	if err != nil {
		return false, err
	}
	return !isRecovering, nil
}

// HealthCheck returns if the server is functioning properly.
func (db *DB) HealthCheck(ctx context.Context) error {
	ctx, task := trace.NewTask(ctx, "db.HealthCheck")
	defer task.End()

	rv := strings.Repeat("dbhealthchecking", 4)
	var payload [1024]byte
	_, err := rand.Read(payload[:])
	if err != nil {
		return err
	}

	insertTime := time.Now().UTC()

	err = db.CreatePartition(ctx, insertTime)
	if err != nil {
		return err
	}

	_, err = db.db.Exec(ctx, "INSERT INTO data VALUES ($1, $2, $3, now());",
		rv, payload[:], insertTime.Format(time.DateOnly))
	if err != nil {
		return err
	}
	var res []byte
	err = db.db.QueryRow(ctx, "SELECT payload FROM data WHERE rendezvous_point = $1", rv).Scan(&res)
	if err != nil {
		// attempt to delete
		db.db.Exec(ctx, "DELETE FROM data WHERE rendezvous_point = $1", rv)
		return err
	}
	_, err = db.db.Exec(ctx, "DELETE FROM data WHERE rendezvous_point = $1", rv)
	if err != nil {
		return err
	}

	if !bytes.Equal(payload[:], res) {
		return fmt.Errorf("payloads do not match")
	}
	return nil
}

// StorePayload stores the provided payload at the given rendezvous point along
// with the given insert time, which typically should typically just be the
// current time.  The provided insert time will be converted to UTC if needed.
// It is an error to attempt to store data at an existing rendezvous point that
// has not expired.
//
// The data will be stored in the bulk data tablespace while the associated
// index data later used to efficiently fetch the data will be stored in the
// index tablespace.
func (db *DB) StorePayload(ctx context.Context, rendezvous ratchet.RVPoint,
	payload []byte, insertTime time.Time) error {

	ctx, task := trace.NewTask(ctx, "storePayload")
	defer task.End()

	// Create a concrete data parition for the day associated with the provided
	// date if needed.
	insertTime = insertTime.UTC()
	db.partitionMtx.Lock()
	if err := db.maybeCreatePartitions(ctx, insertTime); err != nil {
		db.partitionMtx.Unlock()
		return err
	}
	db.partitionMtx.Unlock()

	for attempts := 0; attempts < 2; attempts++ {
		const query = "INSERT INTO data " +
			"(rendezvous_point, payload, insert_time, insert_ts) " +
			"VALUES ($1, $2, $3, $4);"
		_, err := db.db.Exec(ctx, query, rendezvous.String(), payload,
			insertTime, insertTime)
		if err != nil {

			// Create the data partition and go back to the top of the loop to
			// try the insert again if the error indicates the partition doesn't
			// exist.
			//
			// This should pretty much never be hit in practice, but it is
			// technically possible for the partition to have been removed from
			// another connection to the database which would cause the local
			// map of known partitions to be out of sync.  This gracefully
			// handles that scenario.
			var e *pgconn.PgError
			isPQErr := errors.As(err, &e)
			if isPQErr && e.Code == pgerrcode.CheckViolation &&
				attempts == 0 {

				// Ensure the data partition is no longer marked as known to
				// exist to ensure an attempt to create it is made.
				partitionName := partitionTableName("data", insertTime)
				db.partitionMtx.Lock()
				delete(db.dataPartitions, partitionName)
				if err := db.maybeCreatePartitions(ctx, insertTime); err != nil {
					db.partitionMtx.Unlock()
					return err
				}
				db.partitionMtx.Unlock()
				continue
			}

			// A unique violation constraint when attempting to store
			// a duplicate payload is a logical error that is signalled
			// by a specific error.
			if isPQErr && e.Code == pgerrcode.UniqueViolation &&
				(strings.Contains(e.ConstraintName, "_rendezvous_point_insert_time_key") ||
					e.ConstraintName == "global_unique_data_rendezvous") {
				return fmt.Errorf("RV %s: %w", rendezvous, serverdb.ErrAlreadyStoredRV)
			}

			str := fmt.Sprintf("unable to store payload: %v", err)
			return contextError(ErrQueryFailed, str, err)
		}
		break
	}
	return nil
}

// FetchPayload attempts to load the payload at the provided rendezvous point
// along with the time it was inserted.
//
// When there is no payload available at the provided rendezvous point, rather
// than returning an error which would require an allocation, nil is returned.
//
// In other words, callers must check if the result is nil to determine when no
// payload is available at the provided rendezvous point.
func (db *DB) FetchPayload(ctx context.Context, rendezvous ratchet.RVPoint) (*serverdb.FetchPayloadResult, error) {
	ctx, task := trace.NewTask(ctx, "fetchPayload")
	defer task.End()

	const query = "SELECT payload, insert_ts FROM data WHERE " +
		"rendezvous_point = $1 ORDER BY insert_time DESC LIMIT 1;"
	row := db.db.QueryRow(ctx, query, rendezvous.String())
	var payload []byte
	var timestamp time.Time
	if err := row.Scan(&payload, &timestamp); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		str := fmt.Sprintf("unable to fetch payload: %v", err)
		return nil, contextError(ErrQueryFailed, str, err)
	}

	return &serverdb.FetchPayloadResult{
		Payload:    payload,
		InsertTime: timestamp,
	}, nil
}

// RemovePayload removes the payload at the provided rendezvous point if it
// exists.
func (db *DB) RemovePayload(ctx context.Context, rendezvous ratchet.RVPoint) error {
	ctx, task := trace.NewTask(ctx, "removePayload")
	defer task.End()

	const query = "DELETE FROM data WHERE rendezvous_point = $1;"
	_, err := db.db.Exec(ctx, query, rendezvous.String())
	if err != nil {
		str := fmt.Sprintf("unable to remove payload: %v", err)
		return contextError(ErrQueryFailed, str, err)
	}
	return nil
}

// IsSubscriptionPaid returns true if the subscription for the given rendezvous
// point was marked as paid in the DB.
func (db *DB) IsSubscriptionPaid(ctx context.Context, rendezvous ratchet.RVPoint) (bool, error) {
	ctx, task := trace.NewTask(ctx, "isSubscriptionPaid")
	defer task.End()

	const query = "SELECT count(*) FROM paid_subs WHERE rendezvous_point = $1;"
	row := db.db.QueryRow(ctx, query, rendezvous.String())
	var count int
	if err := row.Scan(&count); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		str := fmt.Sprintf("unable to fetch subscription payment status: %v", err)
		return false, contextError(ErrQueryFailed, str, err)
	}

	return count > 0, nil
}

// StoreSubscriptionPaid marks the specified rendezvous as paid in the
// specified date (which tipically will be the current time). It is not an
// error to pay multiple times for the same rendezvous.
func (db *DB) StoreSubscriptionPaid(ctx context.Context, rendezvous ratchet.RVPoint, insertTime time.Time) error {
	ctx, task := trace.NewTask(ctx, "isSubscriptionPaid")
	defer task.End()

	// Create a concrete data parition for the day associated with the provided
	// date if needed.
	insertTime = insertTime.UTC()
	db.partitionMtx.Lock()
	if err := db.maybeCreatePartitions(ctx, insertTime); err != nil {
		db.partitionMtx.Unlock()
		return err
	}
	db.partitionMtx.Unlock()

	for attempts := 0; attempts < 2; attempts++ {
		const query = "INSERT INTO paid_subs" +
			"(rendezvous_point, insert_time, insert_ts) " +
			"VALUES ($1, $2, $3);"
		_, err := db.db.Exec(ctx, query, rendezvous.String(), insertTime, insertTime)
		if err != nil {
			// Create the paid_subs partition and go back to the top of the loop to
			// try the insert again if the error indicates the partition doesn't
			// exist.
			//
			// This should pretty much never be hit in practice, but it is
			// technically possible for the partition to have been removed from
			// another connection to the database which would cause the local
			// map of known partitions to be out of sync.  This gracefully
			// handles that scenario.
			var e *pgconn.PgError
			isPQErr := errors.As(err, &e)
			if isPQErr && e.Code == pgerrcode.CheckViolation &&
				attempts == 0 {

				// Ensure the paid_subs partition is no longer marked as known to
				// exist to ensure an attempt to create it is made.
				partitionName := partitionTableName("paid_subs", insertTime)
				db.partitionMtx.Lock()
				delete(db.paidSubsPartitions, partitionName)
				if err := db.maybeCreatePartitions(ctx, insertTime); err != nil {
					db.partitionMtx.Unlock()
					return err
				}
				db.partitionMtx.Unlock()
				continue
			}

			// A unique violation constraint when attempting to mark
			// a subscription as paid is not a logical error in this
			// software, it's simply regarded as a NOP. So return
			// without error in this case.
			if isPQErr && e.Code == pgerrcode.UniqueViolation &&
				strings.Contains(e.ConstraintName, "_rendezvous_point_insert_time_key") {
				return nil
			}

			str := fmt.Sprintf("unable to mark subscription paid: %v", err)
			return contextError(ErrQueryFailed, str, err)
		}
		break
	}
	return nil

}

// dropTable drops the provided table name from the database if it exists.  It
// is up to the caller to ensure the provided table name is quoted properly via
// pg.QuoteIdentifier.
func dropTable(ctx context.Context, tx pgx.Tx, tableName string) error {
	const query = "DROP TABLE IF EXISTS %s;"
	_, err := tx.Exec(ctx, fmt.Sprintf(query, tableName))
	if err != nil {
		str := fmt.Sprintf("unable to drop table: %v", err)
		return contextError(ErrQueryFailed, str, err)
	}
	return nil
}

// expireTablePartition drops the partition of the given base virtual table
// that was created for the specified date.
func (db *DB) expireTablePartition(ctx context.Context, baseTableName string, date time.Time) (uint64, error) {

	var count uint64
	partitionName := partitionTableName(baseTableName, date)
	err := db.sqlTx(ctx, func(tx pgx.Tx) error {
		// Ensure exclusive access to the data table when modifying or reading
		// partitions from the database.
		queryLock := fmt.Sprintf("LOCK TABLE %s", pq.QuoteIdentifier(baseTableName))
		_, err := tx.Exec(ctx, queryLock)
		if err != nil {
			str := fmt.Sprintf("unable to lock %s table: %v",
				baseTableName, err)
			return contextError(ErrQueryFailed, str, err)
		}

		// Ensure the table still exists since it's possible another backend
		// removed it which would cause the internal map to be out of sync.
		exists, err := tableExists(ctx, tx, partitionName)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		// Count how many entries are being expired.
		const query = "SELECT COUNT(*) FROM %s;"
		tableName := pq.QuoteIdentifier(partitionName)
		row := tx.QueryRow(ctx, fmt.Sprintf(query, tableName))
		if err := row.Scan(&count); err != nil {
			str := fmt.Sprintf("unable to query partition count: %v", err)
			return contextError(ErrQueryFailed, str, err)
		}

		// Drop the table that houses the partition.
		return dropTable(ctx, tx, tableName)
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

// IsPushPaymentRedeemed returns whether the payment done with the specified ID
// was recorded in the database (i.e. redeemed).
func (db *DB) IsPushPaymentRedeemed(ctx context.Context, payID []byte) (bool, error) {
	ctx, task := trace.NewTask(ctx, "isPushPaymentRedeemed")
	defer task.End()

	const query = "SELECT count(*) FROM redeemed_push_payments WHERE payment_id = $1;"
	row := db.db.QueryRow(ctx, query, hex.EncodeToString(payID))
	var count int
	if err := row.Scan(&count); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		str := fmt.Sprintf("unable to fetch subscription payment status: %v", err)
		return false, contextError(ErrQueryFailed, str, err)
	}

	return count > 0, nil
}

// StorePushPaymentRedeemed stores the passed payment ID as redeemed at the
// passed date.
func (db *DB) StorePushPaymentRedeemed(ctx context.Context, payID []byte, insertTime time.Time) error {
	ctx, task := trace.NewTask(ctx, "storePushPaymentRedeemed")
	defer task.End()

	// Create a concrete data parition for the day associated with the provided
	// date if needed.
	insertTime = insertTime.UTC()
	db.partitionMtx.Lock()
	if err := db.maybeCreatePartitions(ctx, insertTime); err != nil {
		db.partitionMtx.Unlock()
		return err
	}
	db.partitionMtx.Unlock()

	hexID := hex.EncodeToString(payID)
	for attempts := 0; attempts < 2; attempts++ {
		const query = "INSERT INTO redeemed_push_payments" +
			"(payment_id, insert_time) " +
			"VALUES ($1, $2);"
		_, err := db.db.Exec(ctx, query, hexID, insertTime)
		if err != nil {
			// Create the redeemed_push_payments partition and go
			// back to the top of the loop to try the insert again
			// if the error indicates the partition doesn't exist.
			//
			// This should pretty much never be hit in practice, but it is
			// technically possible for the partition to have been removed from
			// another connection to the database which would cause the local
			// map of known partitions to be out of sync.  This gracefully
			// handles that scenario.
			var e *pgconn.PgError
			isPQErr := errors.As(err, &e)
			if isPQErr && e.Code == pgerrcode.CheckViolation &&
				attempts == 0 {

				// Ensure the redeemed_push_payments partition
				// is no longer marked as known to exist to
				// ensure an attempt to create it is made.
				partitionName := partitionTableName("redeemed_push_payments", insertTime)
				db.partitionMtx.Lock()
				delete(db.redeemedPushesPartitions, partitionName)
				if err := db.maybeCreatePartitions(ctx, insertTime); err != nil {
					db.partitionMtx.Unlock()
					return err
				}
				db.partitionMtx.Unlock()
				continue
			}

			// A unique violation constraint when attempting to
			// mark a push payment as redeemed is not a logical
			// error in this software, it's simply regarded as a
			// NOP. So return without error in this case.
			if isPQErr && e.Code == pgerrcode.UniqueViolation &&
				strings.Contains(e.ConstraintName, "_payment_id_insert_time_key") {
				return nil
			}

			str := fmt.Sprintf("unable to mark push payment as redeemed: %v", err)
			return contextError(ErrQueryFailed, str, err)
		}
		break
	}
	return nil
}

// Expire removes all entries that were inserted on the same day as the day
// associated with the provided date.  The provided date will be converted to
// UTC if needed.  It returns the number of entries that were removed.
func (db *DB) Expire(ctx context.Context, date time.Time) (uint64, error) {
	ctx, task := trace.NewTask(ctx, "expire")
	defer task.End()

	date = date.UTC()

	db.partitionMtx.Lock()
	defer db.partitionMtx.Unlock()

	// Drop the data partition if it exists.
	var count uint64
	dataPartitionName := partitionTableName("data", date)
	if _, exists := db.dataPartitions[dataPartitionName]; exists {
		var err error
		count, err = db.expireTablePartition(ctx, "data", date)
		if err != nil {
			return 0, err
		}

		// The partition no longer exists.
		delete(db.dataPartitions, dataPartitionName)
	}

	// Drop the paid subscription partition if it exists.
	paidSubPartitionName := partitionTableName("paid_subs", date)
	if _, exists := db.paidSubsPartitions[paidSubPartitionName]; exists {
		_, err := db.expireTablePartition(ctx, "paid_subs", date)
		if err != nil {
			return 0, err
		}

		// The partition no longer exists.
		delete(db.paidSubsPartitions, paidSubPartitionName)
	}

	// Drop the redeemed push pyaments partition if it exists.
	redeemedPushPayName := partitionTableName("redeemed_push_payments", date)
	if _, exists := db.redeemedPushesPartitions[redeemedPushPayName]; exists {
		_, err := db.expireTablePartition(ctx, "redeemed_push_payments", date)
		if err != nil {
			return 0, err
		}

		// The partition no longer exists.
		delete(db.redeemedPushesPartitions, redeemedPushPayName)
	}

	return count, nil
}

// TableSpacesSizes returns the disk size (in bytes) occupied by the bulk and
// index tablespaces (respectively) as reported by the underlying db.
func (db *DB) TableSpacesSizes(ctx context.Context) (uint64, uint64, error) {
	var bulkSize, indexSize uint64
	err := db.sqlTx(ctx, func(tx pgx.Tx) error {
		const query = "select pg_tablespace_size($1), pg_tablespace_size($2)"
		row := tx.QueryRow(ctx, query, db.bulkDataTablespace,
			db.indexTablespace)
		if err := row.Scan(&bulkSize, &indexSize); err != nil {
			str := fmt.Sprintf("unable to query tablespaces size: %v",
				err)
			return contextError(ErrQueryFailed, str, err)
		}
		return nil
	})

	return bulkSize, indexSize, err
}

// Close closes the backend and prevents new queries from starting.  It then
// waits for all queries that have started processing on the server to finish.
func (db *DB) Close() {
	db.db.Close()
}

// options houses the configurable values when creating a backend.
type options struct {
	host               string
	port               string
	dbName             string
	roleName           string
	passphrase         string
	sslMode            string
	serverCA           string
	indexTablespace    string
	bulkDataTablespace string
}

// Option represents a modification to the configuration parameters used by
// Open.
type Option func(*options)

// WithHost overrides the default host for the host that serves the backing
// database with a custom value.
//
// The host may be an IP address for TCP connection, or an absolute path to a
// UNIX domain socket.  In the case UNIX sockets are used, the port should also
// be set to an empty string via WithPort.
func WithHost(host string) Option {
	return func(o *options) {
		o.host = host
	}
}

// WithPort overrides the default port for the host that serves the backing
// database with a custom value.
func WithPort(port string) Option {
	return func(o *options) {
		o.port = port
	}
}

// WithDBName overrides the default name for the backing database with a custom
// value.
func WithDBName(dbName string) Option {
	return func(o *options) {
		o.dbName = dbName
	}
}

// WithRole overrides the default role name that is used to access the database
// with a custom value.
func WithRole(roleName string) Option {
	return func(o *options) {
		o.roleName = roleName
	}
}

// WithPassphrase overrides the default passphrase that is used to access the
// database with a custom value.
func WithPassphrase(passphrase string) Option {
	return func(o *options) {
		o.passphrase = passphrase
	}
}

// WithIndexTablespace overrides the default name of the tablespace that is used
// to store indexes with a custom value.
func WithIndexTablespace(tablespace string) Option {
	return func(o *options) {
		o.indexTablespace = tablespace
	}
}

// WithIndexTablespace overrides the default name of the tablespace that is used
// to store the bulk payload data with a custom value.
func WithBulkDataTablespace(tablespace string) Option {
	return func(o *options) {
		o.bulkDataTablespace = tablespace
	}
}

// WithTLS connects to the backing database with TLS and verifies that the
// certificate presented by the server was signed by the provided CA, which is
// typically the server certicate itself for self-signed certificates, and that
// the server host name matches the one in the certificate.
//
// The provided server CA can be an empty string to use the system CAs instead
// for certs that are signed by one of them.
func WithTLS(serverCA string) Option {
	return func(o *options) {
		o.sslMode = "verify-full"
		o.serverCA = serverCA
	}
}

// Open opens a connection to a database, potentially creates any necessary data
// tables and partitions as needed, and returns a backend instance that is safe
// for concurrent use.
//
// Callers are responsible for calling Close on the returned instance when
// finished using it to ensure a clean shutdown.
//
// Use the functions that start with the prefix "With" to provide configuration
// operations.
//
// For example:
//
//	db, err := pgdb.Open(ctx, pgdb.WithHost(host), pgdb.WithPassphrase(pass)
//		pgdb.WithTLS("./server.crt"))
//	if err != nil {
//		/* handle err */
//	}
//	defer db.Close()
func Open(ctx context.Context, opts ...Option) (*DB, error) {
	// Configuration options.
	o := options{
		host:               DefaultHost,
		port:               DefaultPort,
		dbName:             DefaultDBName,
		roleName:           DefaultRoleName,
		passphrase:         DefaultRoleName, // Same as the role name.
		sslMode:            "disable",
		indexTablespace:    DefaultIndexTablespaceName,
		bulkDataTablespace: DefaultBulkDataTablespaceName,
	}
	for _, f := range opts {
		f(&o)
	}

	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=%s "+
		"application_name=brserver target_session_attrs=any",
		o.host, o.roleName, o.passphrase, o.dbName, o.sslMode)
	if !strings.HasPrefix(o.host, "/") {
		connStr += fmt.Sprintf(" port=%s", o.port)
	}
	if o.sslMode != "disable" && o.serverCA != "" {
		connStr += fmt.Sprintf(" sslrootcert='%s'", o.serverCA)
	}

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		str := fmt.Sprintf("failed to create connection config: %v", err)
		return nil, contextError(ErrConnFailed, str, err)
	}

	db := &DB{
		dbName:                   o.dbName,
		roleName:                 o.roleName,
		indexTablespace:          o.indexTablespace,
		bulkDataTablespace:       o.bulkDataTablespace,
		dataPartitions:           make(map[string]struct{}),
		paidSubsPartitions:       make(map[string]struct{}),
		redeemedPushesPartitions: make(map[string]struct{}),
	}

	afterConnect := func(ctx context.Context, sqlDB *pgx.Conn) error {
		if err := sqlDB.Ping(ctx); err != nil {
			str := fmt.Sprintf("unable to communicate with database: %v", err)
			return contextError(ErrConnFailed, str, err)
		}

		// This ensures proper behavior in the case multiple connections are opened
		// to the same backend.
		db.initMtx.Lock()
		defer db.initMtx.Unlock()

		isMaster, err := isMaster(ctx, sqlDB)
		if err != nil {
			return err
		}

		// Initialize or update the database, populate the local state, and check
		// the overall sanity.  Sanity checks include things such as having the
		// required settings configured, existence of required tables along with
		// being configured with their expected tablespaces.
		if isMaster {
			err = sqlTxInternal(ctx, sqlDB, func(tx pgx.Tx) error {
				if err := db.checkDatabaseInitialized(ctx, tx); err != nil {
					return err
				}
				if err = db.initDB(ctx, tx); err != nil {
					return err
				}
				return db.checkDatabaseSanity(ctx, tx)
			})
		} else {
			err = sqlTxROInternal(ctx, sqlDB, func(tx pgx.Tx) error {
				if err := db.checkDatabaseInitialized(ctx, tx); err != nil {
					return err
				}
				return db.checkDatabaseSanity(ctx, tx)
			})
		}
		if err != nil {
			return err
		}
		return nil
	}
	poolCfg.AfterConnect = afterConnect

	sqlDB, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		str := fmt.Sprintf("unable to open connection to database: %v", err)
		return nil, contextError(ErrConnFailed, str, err)
	}
	db.db = sqlDB

	return db, nil
}
