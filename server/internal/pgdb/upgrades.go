package pgdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// upgradeDBToV2 upgrades the database to V2. This involves renaming the old
// insert_time field to insert_date and adding a new insert_ts field.
func upgradeDBToV2(ctx context.Context, tx *sql.Tx, dbInfo *databaseInfo) error {
	if dbInfo.version != 1 {
		str := fmt.Sprintf("cannot upgrade db to version 2 from version %d",
			dbInfo.version)
		return contextError(ErrUpgradeV2, str, nil)
	}

	tablePrefixes := []string{"data", "paid_subs"}
	for _, tableName := range tablePrefixes {
		// Add the new column.
		queryAddTSField := fmt.Sprintf("ALTER TABLE %s"+
			"ADD COLUMN insert_ts TIMESTAMP NOT NULL DEFAULT current_timestamp;",
			pq.QuoteIdentifier(tableName))
		_, err := tx.ExecContext(ctx, queryAddTSField)
		if err != nil {
			str := fmt.Sprintf("unable to add insert_ts field to %s table: %v",
				tableName, err)
			return contextError(ErrUpgradeV2, str, err)
		}
	}

	return nil
}

// upgradeDBToV3 upgrades the database to V3. This involves adding the new
// virtual table to store payment redemption status for pushed messages.
func upgradeDBToV3(ctx context.Context, tx *sql.Tx, dbInfo *databaseInfo, indexTablespace string) error {
	if dbInfo.version != 2 {
		str := fmt.Sprintf("cannot upgrade db to version 3 from version %d",
			dbInfo.version)
		return contextError(ErrUpgradeV3, str, nil)
	}

	const queryTemplate = "CREATE TABLE IF NOT EXISTS redeemed_push_payments (" +
		"	payment_id TEXT NOT NULL," +
		"	insert_time DATE NOT NULL," +
		"	UNIQUE(payment_id, insert_time) USING INDEX TABLESPACE %s" +
		") PARTITION BY RANGE (insert_time);"
	tablespace := pq.QuoteIdentifier(indexTablespace)
	query := fmt.Sprintf(queryTemplate, tablespace)

	_, err := tx.Exec(query)
	if err != nil {
		str := fmt.Sprintf("unable to create redeemed push payments table: %v", err)
		return contextError(ErrUpgradeV3, str, err)
	}

	return nil
}
