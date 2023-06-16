package clientdb

import "path/filepath"

// StoreContentFilter stores the given filter in the DB.
func (db *DB) StoreContentFilter(tx ReadWriteTx, filter *ContentFilter) error {
	baseDir := filepath.Join(db.root, filtersDir)

	if filter.ID == 0 {
		last, err := filtersFnamePattern.Last(baseDir)
		if err != nil {
			return err
		}
		filter.ID = last.ID + 1
	}

	fname := filepath.Join(baseDir, filtersFnamePattern.FilenameFor(filter.ID))
	return db.saveJsonFile(fname, filter)
}

// RemoveContentFilter removes the filter with the specified ID.
func (db *DB) RemoveContentFilter(tx ReadWriteTx, filterID uint64) error {
	baseDir := filepath.Join(db.root, filtersDir)
	fname := filepath.Join(baseDir, filtersFnamePattern.FilenameFor(filterID))
	return removeIfExists(fname)
}

// ListContentFilters returns all content filters.
func (db *DB) ListContentFilters(tx ReadTx) ([]ContentFilter, error) {
	baseDir := filepath.Join(db.root, filtersDir)
	files, err := filtersFnamePattern.MatchFiles(baseDir)
	if err != nil {
		return nil, err
	}

	res := make([]ContentFilter, 0, len(files))
	for _, f := range files {
		fname := filepath.Join(baseDir, f.Filename)
		var cf ContentFilter
		if err := db.readJsonFile(fname, &cf); err != nil {
			db.log.Warnf("Unable to read content filter file %s: %v",
				fname, err)
			continue
		}

		res = append(res, cf)
	}

	return res, nil
}
