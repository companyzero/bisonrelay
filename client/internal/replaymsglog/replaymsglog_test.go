package replaymsglog

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
	"golang.org/x/exp/rand"
)

type testStruct struct {
	Field string `json:"field"`
}

// TestCorrectBehavior ensure the msg logger behaves correctly.
func TestCorrectBehavior(t *testing.T) {
	maxSize := 1 << 10 // 1KiB / file
	cfg := Config{
		RootDir: testutils.TempTestDir(t, "replaymsglog"),
		Prefix:  "rl",
		MaxSize: uint32(maxSize),
		//Log:     testutils.TestLoggerSys(t, "XXXX"),
	}
	rl, err := New(cfg)
	assert.NilErr(t, err)

	sampleFieldVal := strings.Repeat(" ", 100)
	v := testStruct{Field: sampleFieldVal}
	encodedV, _ := json.Marshal(v)
	vSize := len(encodedV) + 1 + 5 // v + \n + XXXXX suffix in .Field

	// Generate enough data for 4 files.
	nbEntries := (maxSize * 3) / vSize
	allIDs := make([]ID, 0, nbEntries)
	for i := 0; i < nbEntries; i++ {
		v.Field = sampleFieldVal + fmt.Sprintf("%05d", i)
		id, err := rl.Store(v)
		assert.NilErr(t, err)
		allIDs = append(allIDs, id)
	}

	// Last ID should have file index of 4.
	gotFID := allIDs[len(allIDs)-1].fileIndex()
	if gotFID != 4 {
		t.Fatalf("unexpected fileIndex() on last id: %d", gotFID)
	}

	// Helper to assert ReadAfter works as expected when we start to read
	// from a given index.
	assertReadAfterWorks := func(startIndex int) {
		t.Helper()

		var startID ID
		wantIndex := startIndex + 1
		wantLastIndex := nbEntries
		if startIndex >= nbEntries {
			startID = ID(math.MaxUint64)
			wantLastIndex = wantIndex
		} else if startIndex > 0 {
			startID = allIDs[startIndex]
		} else {
			wantIndex = 0
		}

		var v testStruct
		err := rl.ReadAfter(startID, &v, func(gotID ID) error {
			wantField := sampleFieldVal + fmt.Sprintf("%05d", wantIndex)
			if v.Field != wantField {
				return fmt.Errorf("unexpected field value when asserting %d "+
					"after %d: got %s, want %s",
					wantIndex, startIndex, v.Field, wantField)
			}
			if gotID != allIDs[wantIndex] {
				return fmt.Errorf("unexpected ID when asserting %d "+
					"after %d: got %s, want %s",
					wantIndex, startIndex, gotID, allIDs[wantIndex])
			}
			wantIndex += 1
			return nil
		})
		assert.NilErr(t, err)

		if wantIndex != wantLastIndex {
			t.Fatalf("unexpected final index when asserting "+
				"after %d: got %d, want %d",
				startIndex, wantIndex, wantLastIndex)
		}
	}

	// Assert reading from each ID works as expected. -1 means and +1 make
	// it test for cases before the first ID and after the first ID.
	for i := -1; i < nbEntries+1; i++ {
		assertReadAfterWorks(i)
	}

	// Delete one by one, up to half of all entries, asserting the
	// behavior of ReadAfter after each deletion.
	for i := 0; i < nbEntries/2; i++ {
		err := rl.ClearUpTo(allIDs[i])
		assert.NilErr(t, err)

		for i := i + 1; i < nbEntries+1; i++ {
			assertReadAfterWorks(i)
		}
	}

	// Add another entry after deleting some.
	v.Field = sampleFieldVal + fmt.Sprintf("%05d", nbEntries)
	id, err := rl.Store(v)
	assert.NilErr(t, err)
	allIDs = append(allIDs, id)
	nbEntries += 1

	// Recreate the replay log, with the same config (simulating a restart).
	rl, err = New(cfg)
	assert.NilErr(t, err)

	// Add another entry after restarting.
	v.Field = sampleFieldVal + fmt.Sprintf("%05d", nbEntries)
	id, err = rl.Store(v)
	assert.NilErr(t, err)
	allIDs = append(allIDs, id)
	nbEntries += 1

	// Delete everything up to a random ID.
	indexToDelete := nbEntries - 8
	err = rl.ClearUpTo(allIDs[indexToDelete])
	assert.NilErr(t, err)
	for i := indexToDelete; i < nbEntries+1; i++ {
		assertReadAfterWorks(i)
	}
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// TestConcurrentReplayMsgLog tests that storing, reading and clearing
// concurrently works.
func TestConcurrentReplayMsgLog(t *testing.T) {
	maxSize := 1 << 10 // 1KiB / file
	cfg := Config{
		RootDir: testutils.FixedTempDir(t, "replaymsglog"),
		Prefix:  "rl",
		MaxSize: uint32(maxSize),
		//Log:     testutils.TestLoggerSys(t, "XXXX"),
	}
	rl, err := New(cfg)
	assert.NilErr(t, err)

	testTime := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), testTime)
	defer cancel()

	nbReaders := 8
	idChan := make(chan ID, nbReaders*2)
	delChan := make(chan ID, nbReaders)

	go func() {
		for !isDone(ctx) {
			v := testStruct{Field: "sample field"}
			id, err := rl.Store(v)
			if err != nil {
				panic(err)
			}
			idChan <- id
		}
	}()
	for i := 0; i < nbReaders; i++ {
		go func() {
			var v testStruct
			for {
				select {
				case <-ctx.Done():
					return
				case id := <-idChan:
					err := rl.ReadAfter(id, &v, func(ID) error { return nil })
					if err != nil {
						panic(err)
					}
					time.Sleep(time.Duration(rand.Intn(1000)) * time.Microsecond)
					delChan <- id
					time.Sleep(time.Duration(rand.Intn(1000)) * time.Microsecond)
				}
			}
		}()
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case id := <-idChan:
				err := rl.ClearUpTo(id)
				if err != nil {
					panic(err)
				}
			}
		}
	}()
	<-ctx.Done()
}

// TestConcurrentReadClear tests that clearing while reading works as intended.
func TestConcurrentReadClear(t *testing.T) {
	maxSize := 1 << 10 // 1KiB / file
	cfg := Config{
		RootDir: testutils.TempTestDir(t, "replaymsglog"),
		Prefix:  "rl",
		MaxSize: uint32(maxSize),
		//Log:     testutils.TestLoggerSys(t, "XXXX"),
	}
	rl, err := New(cfg)
	assert.NilErr(t, err)

	// Store a number of entries.
	const nbEntries = 100
	var testID ID
	sampleFieldVal := strings.Repeat(" ", 100)
	for i := 0; i < nbEntries; i++ {
		v := testStruct{Field: sampleFieldVal + fmt.Sprintf("%05d", i)}
		id, err := rl.Store(v)
		assert.NilErr(t, err)
		if i == nbEntries/2 {
			testID = id
		}
	}

	// Start reading and clear up to a test ID during reading.
	var v testStruct
	var readIDs []ID
	err = rl.ReadAfter(testID, &v, func(id ID) error {
		readIDs = append(readIDs, id)
		return rl.ClearUpTo(testID)
	})
	assert.NilErr(t, err)

	// Read again starting at the test ID, and verify all entries are still
	// there.
	var i int
	err = rl.ReadAfter(0, &v, func(id ID) error {
		if i >= len(readIDs) {
			return fmt.Errorf("too few readIDs: %d", i)
		}
		if readIDs[i] != id {
			return fmt.Errorf("unexpected readID: got %s, want %s",
				id, readIDs[i])
		}
		i += 1
		return nil
	})
	assert.NilErr(t, err)
	if i != len(readIDs) {
		t.Fatalf("too many readIDs: got %d, want %d", i, len(readIDs))
	}
}

// TestClearEmptyLog tests that attempting to clear an empty log does not fail.
func TestClearEmptyLog(t *testing.T) {
	maxSize := 1 << 10 // 1KiB / file
	cfg := Config{
		RootDir: testutils.TempTestDir(t, "replaymsglog"),
		Prefix:  "rl",
		MaxSize: uint32(maxSize),
		//Log:     testutils.TestLoggerSys(t, "XXXX"),
	}
	rl, err := New(cfg)
	assert.NilErr(t, err)
	err = rl.ClearUpTo(makeID(100, 200))
	assert.NilErr(t, err)
}
