package main

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/server/internal/pgdb"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
)

func humanReadableBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

type rvSorter [][32]byte

func (s rvSorter) Len() int           { return len(s) }
func (s rvSorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s rvSorter) Less(i, j int) bool { return bytes.Compare(s[i][:], s[j][:]) < 0 }

type asyncRVs struct {
	mtx      sync.Mutex
	numSaved uint64
	rvs      [][32]byte
}

type simulator struct {
	minChunkSize       uint64
	maxChunkSize       uint64
	daysToSimulate     uint64
	totalBytesToInsert uint64
	noExpiration       bool
	db                 *pgdb.DB
	startDay           time.Time

	randomRVs asyncRVs
}

const queriesPerDay = 1500
const maxDayDuration = int64(time.Hour*23 + time.Minute*59 + time.Second*59)

func (s *simulator) paySubHandler(ctx context.Context, rng *rand.Rand,
	timeInserted time.Time, total uint64) error {
	for i := uint64(0); i < total; i++ {
		var rv [32]byte
		if _, err := rng.Read(rv[:]); err != nil {
			return err
		}

		s.randomRVs.mtx.Lock()
		if s.randomRVs.numSaved < queriesPerDay {
			s.randomRVs.rvs = append(s.randomRVs.rvs, rv)
			s.randomRVs.numSaved++
		}
		s.randomRVs.mtx.Unlock()

		// Insert with a random time within the specified day.
		randDur := rng.Int63n(maxDayDuration)
		recordTime := timeInserted.Add(time.Duration(randDur))

		// Store the payload at the provided rv.
		err := s.db.StoreSubscriptionPaid(ctx, rv, recordTime)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *simulator) redeemedPaidPushesHandler(ctx context.Context, rng *rand.Rand,
	timeInserted time.Time, total uint64) error {
	for i := uint64(0); i < total; i++ {
		var id [32]byte
		if _, err := rng.Read(id[:]); err != nil {
			return err
		}

		s.randomRVs.mtx.Lock()
		if s.randomRVs.numSaved < queriesPerDay {
			s.randomRVs.rvs = append(s.randomRVs.rvs, id)
			s.randomRVs.numSaved++
		}
		s.randomRVs.mtx.Unlock()

		// Insert with a random time within the specified day.
		randDur := rng.Int63n(maxDayDuration)
		recordTime := timeInserted.Add(time.Duration(randDur))

		// Store the payload at the provided rv.
		err := s.db.StorePushPaymentRedeemed(ctx, id[:], recordTime)
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *simulator) genStoreHandler(ctx context.Context, rng *rand.Rand, timeInserted time.Time, c <-chan uint64) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case payloadSize, ok := <-c:
			if !ok {
				return nil
			}

			// Create a random payload of the requested size (which is a random
			// size between the min and max chunk size) along with a random
			// simulated rendezvous point.
			payload := make([]byte, payloadSize)
			if _, err := rng.Read(payload); err != nil {
				return err
			}
			var rv [32]byte
			if _, err := rng.Read(rv[:]); err != nil {
				return err
			}
			s.randomRVs.mtx.Lock()
			if s.randomRVs.numSaved < queriesPerDay {
				s.randomRVs.rvs = append(s.randomRVs.rvs, rv)
				s.randomRVs.numSaved++
			}
			s.randomRVs.mtx.Unlock()

			// Insert with a random time within the specified day.
			randDur := rng.Int63n(maxDayDuration)
			recordTime := timeInserted.Add(time.Duration(randDur))

			// Store the payload at the provided rv.
			err := s.db.StorePayload(ctx, rv, payload, recordTime)
			if err != nil {
				return err
			}
		}
	}
}

func (s *simulator) Run(ctx context.Context) error {
	// Split the total bytes to insert into an equal number of bytes per day to
	// simulate.
	totalBytesToInsertPerDay := s.totalBytesToInsert / s.daysToSimulate

	startDay := s.startDay
	numCPU := runtime.NumCPU()
	baseSeed := time.Now().Unix()
	s.randomRVs.rvs = make([][32]byte, 0, queriesPerDay*s.daysToSimulate)

	var totalBytesInserted, totalRecordsInserted uint64
	start := time.Now()
	for day := uint64(0); day < s.daysToSimulate; day++ {
		// Reset count of saved RVs for the day.
		s.randomRVs.mtx.Lock()
		s.randomRVs.numSaved = 0
		s.randomRVs.mtx.Unlock()

		timeInserted := startDay.Add(time.Hour * 24 * time.Duration(day))
		fmt.Printf("Loading %s for %v (day %d of %d)...",
			humanReadableBytes(totalBytesToInsertPerDay),
			timeInserted.Format("2006-01-02"), day+1, s.daysToSimulate)

		// Asynchronously generate and store random payloads of random sizes
		// at random rendezvous points until the total bytes to insert per day
		// has been reached.
		genStoreC := make(chan uint64)
		var bytesInserted, recordsInserted uint64
		g, groupCtx := errgroup.WithContext(ctx)
		startGenAndStore := time.Now()
		for i := 0; i < numCPU; i++ {
			i := i
			g.Go(func() error {
				seed := baseSeed
				seed += (int64(day) * int64(numCPU)) + int64(i)
				rng := rand.New(rand.NewSource(seed))
				return s.genStoreHandler(groupCtx, rng, timeInserted, genStoreC)
			})
		}
		g.Go(func() error {
			rng := rand.New(rand.NewSource(time.Now().Unix()))
			for bytesInserted < totalBytesToInsertPerDay {
				// Create a random payload size between the min and max chunk
				// size.
				maxAdjustedPayload := int64(s.maxChunkSize - s.minChunkSize)
				payloadSize := rng.Int63n(maxAdjustedPayload) + int64(s.minChunkSize)
				select {
				case <-groupCtx.Done():
					return groupCtx.Err()
				case genStoreC <- uint64(payloadSize):
				}
				bytesInserted += uint64(payloadSize)
				recordsInserted++
			}

			close(genStoreC)
			return nil
		})
		if err := g.Wait(); err != nil {
			return err
		}
		elapsedGenAndStore := time.Since(startGenAndStore)

		fmt.Printf("inserted %s (%d records), elapsed %s\n",
			humanReadableBytes(bytesInserted), recordsInserted,
			elapsedGenAndStore.Round(time.Millisecond))

		totalBytesInserted += bytesInserted
		totalRecordsInserted += recordsInserted
	}
	elapsed := time.Since(start)

	fmt.Printf("Total inserted %s (%d records, avg bytes per record: %s), "+
		"total elapsed %s\n",
		humanReadableBytes(totalBytesInserted), totalRecordsInserted,
		humanReadableBytes(totalBytesInserted/totalRecordsInserted),
		elapsed.Round(time.Millisecond))

	// Sort the RVs so there is a good distribution across the partitions.
	s.randomRVs.mtx.Lock()
	randomRVs := s.randomRVs.rvs
	s.randomRVs.rvs = nil
	s.randomRVs.numSaved = 0
	s.randomRVs.mtx.Unlock()
	sort.Sort(rvSorter(randomRVs))

	// Timing for query of a random payload.
	start = time.Now()
	result, err := s.db.FetchPayload(ctx, randomRVs[0])
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	if result != nil {
		fmt.Printf("Query time for random existing rv: %s (insert time: %v, "+
			"payload len: %s)\n", elapsed.Round(time.Millisecond),
			result.InsertTime, humanReadableBytes(uint64(len(result.Payload))))
	}

	// Timing for query of a payload that does not exist.
	var zeroRV [32]byte
	start = time.Now()
	result, err = s.db.FetchPayload(ctx, zeroRV)
	if err != nil {
		return err
	}
	if result != nil {
		fmt.Printf("payload for data at missing rv is not nil (len: %d)\n",
			len(result.Payload))
	}
	elapsed = time.Since(start)
	fmt.Printf("Query time for missing rv: %s\n", elapsed.Round(time.Microsecond))

	// Average timing for query of random payloads.
	fmt.Printf("Querying %d random existing rvs...", len(randomRVs))
	var totalQueriedPayloads uint64
	start = time.Now()
	for i := range randomRVs {
		rv := randomRVs[i]
		result, err := s.db.FetchPayload(ctx, rv)
		if err != nil {
			return err
		}
		if result != nil {
			totalQueriedPayloads += uint64(len(result.Payload))
		}
	}
	elapsed = time.Since(start)
	fmt.Printf("queried %s, elapsed %s, average per query: %s\n",
		humanReadableBytes(totalQueriedPayloads),
		elapsed.Round(time.Millisecond),
		(elapsed / time.Duration(len(randomRVs))).Round(time.Microsecond))

	// Timing registering subscription payments.
	start = time.Now()
	toPayPerCPU := totalRecordsInserted / s.daysToSimulate / uint64(numCPU)
	totalToPay := toPayPerCPU * s.daysToSimulate * uint64(numCPU)
	fmt.Printf("Registering %d subscriptions as paid... ", totalToPay)
	for day := uint64(0); day < s.daysToSimulate; day++ {
		// Reset count of saved RVs for the day.
		s.randomRVs.mtx.Lock()
		s.randomRVs.numSaved = 0
		s.randomRVs.mtx.Unlock()

		timeInserted := startDay.Add(time.Hour * 24 * time.Duration(day))
		g, groupCtx := errgroup.WithContext(ctx)
		for i := 0; i < numCPU; i++ {
			i := i
			g.Go(func() error {
				seed := baseSeed
				seed += (int64(day) * int64(numCPU)) + int64(i)
				rng := rand.New(rand.NewSource(seed))
				return s.paySubHandler(groupCtx, rng, timeInserted, toPayPerCPU)
			})
		}

		if err := g.Wait(); err != nil {
			return fmt.Errorf("unable to mark paid subscriptions: %v", err)
		}
	}
	elapsed = time.Since(start)
	if totalToPay > 0 {
		fmt.Printf("elapsed %v, average per sub %v\n",
			elapsed.Round(time.Millisecond),
			(elapsed / time.Duration(totalToPay)).Round(time.Microsecond))
	}

	// Timing querying for paid subscriptions.
	s.randomRVs.mtx.Lock()
	randomRVs = s.randomRVs.rvs
	s.randomRVs.mtx.Unlock()

	start = time.Now()
	fmt.Printf("Querying %d subs for paid status... ", len(randomRVs))
	for i := range randomRVs {
		rv := randomRVs[i]
		paid, err := s.db.IsSubscriptionPaid(ctx, rv)
		if err != nil {
			return fmt.Errorf("unable to query subscription paid status: %v", err)
		}
		if !paid {
			return fmt.Errorf("unexpected unpaid status for RV %x", rv)
		}
	}
	elapsed = time.Since(start)
	if len(randomRVs) > 0 {
		fmt.Printf("elapsed %v, average per query: %v\n",
			elapsed.Round(time.Millisecond),
			(elapsed / time.Duration(len(randomRVs))).Round(time.Microsecond))
	} else {
		fmt.Printf("\n")
	}

	// Timing registering redeemed push payments.
	start = time.Now()
	toRedeemPerCPU := totalRecordsInserted / s.daysToSimulate / uint64(numCPU)
	totalToRedeem := toPayPerCPU * s.daysToSimulate * uint64(numCPU)
	fmt.Printf("Registering %d push payments as redeemed... ", totalToPay)
	for day := uint64(0); day < s.daysToSimulate; day++ {
		// Reset count of saved RVs for the day.
		s.randomRVs.mtx.Lock()
		s.randomRVs.numSaved = 0
		s.randomRVs.mtx.Unlock()

		timeInserted := startDay.Add(time.Hour * 24 * time.Duration(day))
		g, groupCtx := errgroup.WithContext(ctx)
		for i := 0; i < numCPU; i++ {
			i := i
			g.Go(func() error {
				seed := baseSeed
				seed += (int64(day) * int64(numCPU)) + int64(i)
				rng := rand.New(rand.NewSource(seed))
				return s.redeemedPaidPushesHandler(groupCtx, rng, timeInserted, toRedeemPerCPU)
			})
		}

		if err := g.Wait(); err != nil {
			return fmt.Errorf("unable to mark paid subscriptions: %v", err)
		}
	}
	elapsed = time.Since(start)
	if totalToPay > 0 {
		fmt.Printf("elapsed %v, average per redemption %v\n",
			elapsed.Round(time.Millisecond),
			(elapsed / time.Duration(totalToRedeem)).Round(time.Microsecond))
	}

	// Timing querying for redeemed push payments.
	s.randomRVs.mtx.Lock()
	randomRVs = s.randomRVs.rvs
	s.randomRVs.mtx.Unlock()

	start = time.Now()
	fmt.Printf("Querying %d ids for redeemed status... ", len(randomRVs))
	for i := range randomRVs {
		id := randomRVs[i][:]
		redeemed, err := s.db.IsPushPaymentRedeemed(ctx, id)
		if err != nil {
			return fmt.Errorf("unable to query subscription paid status: %v", err)
		}
		if !redeemed {
			return fmt.Errorf("unexpected unpaid status for RV %x", id)
		}
	}
	elapsed = time.Since(start)
	if len(randomRVs) > 0 {
		fmt.Printf("elapsed %v, average per query: %v\n",
			elapsed.Round(time.Millisecond),
			(elapsed / time.Duration(len(randomRVs))).Round(time.Microsecond))
	} else {
		fmt.Printf("\n")
	}

	// Timing querying for inexistent push payment.
	var randomID [32]byte
	_, _ = cryptorand.Read(randomID[:])
	start = time.Now()
	isRedeemed, err := s.db.IsPushPaymentRedeemed(ctx, randomID[:])
	if err != nil {
		return err
	}
	if isRedeemed {
		fmt.Printf("random ID is marked as redeemed %x\n",
			randomID)
	}
	elapsed = time.Since(start)
	fmt.Printf("Query time for unredeemed push payment: %s\n", elapsed.Round(time.Microsecond))

	// Log final sizes.
	bulkSize, indexSize, err := s.db.TableSpacesSizes(ctx)
	if err != nil {
		return fmt.Errorf("unable to fetch table space sizes: %v", err)
	}
	fmt.Printf("Tablespaces sizes: bulk: %s, index: %s\n",
		humanReadableBytes(bulkSize), humanReadableBytes(indexSize))

	if s.noExpiration {
		fmt.Printf("Not expiring entries as requested\n")
		return nil
	}

	// Timing for bulk expiring all entries for each day.
	var totalExpireElapsed time.Duration
	for day := uint64(0); day < s.daysToSimulate; day++ {
		start := time.Now()
		toExpireDay := startDay.Add(time.Hour * 24 * time.Duration(day))
		numExpired, err := s.db.Expire(ctx, toExpireDay)
		if err != nil {
			return err
		}
		elapsed := time.Since(start)
		fmt.Printf("Time to expire %v: %v (%d expired)\n",
			toExpireDay.Format("2006-01-02"), elapsed.Round(time.Millisecond),
			numExpired)

		totalExpireElapsed += elapsed
	}
	fmt.Printf("Time to expire %d days (%d records): %v\n", s.daysToSimulate,
		totalRecordsInserted, totalExpireElapsed.Round(time.Millisecond))

	return nil
}

func realMain() error {
	var err error

	const (
		defaultDBName       = "brdatasim"
		defaultMinChunkSize = 256
		defaultMaxChunkSize = 1024 * 1024 // 1 MiB
		defaultDays         = 7
		defaultTotalBytes   = 2 * 1024 * 1024 * 1024 // 2 GiB
	)
	var (
		host           = flag.String("host", pgdb.DefaultHost, "database server host")
		port           = flag.String("port", pgdb.DefaultPort, "database server port")
		username       = flag.String("username", pgdb.DefaultRoleName, "database user name")
		dbName         = flag.String("dbname", defaultDBName, "name of the database to use for the simulation")
		noTLS          = flag.Bool("notls", false, "disable TLS")
		serverCA       = flag.String("servercafile", "./server.crt", "path to the file containing Certifcate Authorities to verify the TLS server certificate, ignored with -notls")
		minChunkSize   = flag.Uint64("minchunksize", defaultMinChunkSize, "minimum chunk size for payloads")
		maxChunkSize   = flag.Uint64("maxchunksize", defaultMaxChunkSize, "maximum chunk size for payloads")
		daysToSimulate = flag.Uint64("days", defaultDays, "number of days to simulate")
		totalBytes     = flag.Uint64("totalbytes", defaultTotalBytes, "total number of bytes to insert during simulation")
		noExpiration   = flag.Bool("noexpiration", false, "disable expiring data")

		startDay    = flag.String("startday", "", "simulate data being inserted starting at the specified date (YYYY-MM-DD)")
		indexTSName = flag.String("indextsname", pgdb.DefaultIndexTablespaceName, "name of the tablespace for the indices")
		bulkTSName  = flag.String("bulktsname", pgdb.DefaultBulkDataTablespaceName, "name of the tablespace for the bulk data")
	)
	flag.Parse()

	if *daysToSimulate < 1 {
		return errors.New("-days must be a min of 1")
	}
	now := time.Now().UTC()
	sday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if *startDay != "" {
		sday, err = time.Parse("2006-01-02", *startDay)
		if err != nil {
			return fmt.Errorf("unable to parse start day: %v", err)
		}
	}
	if *daysToSimulate > math.MaxUint8 {
		return fmt.Errorf("-days must be a max of %d", math.MaxUint8)
	}
	if *totalBytes < *daysToSimulate*1024 {
		return fmt.Errorf("-totalbytes must be a min of 1 KiB per day (%d)",
			*daysToSimulate*1024)
	}
	if *totalBytes > math.MaxInt64 {
		return fmt.Errorf("-totalbytes must be a max of %d", math.MaxInt64)
	}
	fmt.Printf("Password for user %s: ", *username)
	passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	fmt.Println()

	ctx := context.Background()
	opts := []pgdb.Option{
		pgdb.WithHost(*host),
		pgdb.WithPort(*port),
		pgdb.WithRole(*username),
		pgdb.WithDBName(*dbName),
		pgdb.WithPassphrase(string(passBytes)),
		pgdb.WithIndexTablespace(*indexTSName),
		pgdb.WithBulkDataTablespace(*bulkTSName),
	}
	if !*noTLS {
		opts = append(opts, pgdb.WithTLS(*serverCA))
	}
	db, err := pgdb.Open(ctx, opts...)
	if err != nil {
		return err
	}
	defer db.Close()

	s := simulator{
		minChunkSize:       *minChunkSize,
		maxChunkSize:       *maxChunkSize,
		daysToSimulate:     *daysToSimulate,
		totalBytesToInsert: *totalBytes,
		noExpiration:       *noExpiration,
		startDay:           sday,
		db:                 db,
	}
	return s.Run(ctx)
}

func main() {
	if err := realMain(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
