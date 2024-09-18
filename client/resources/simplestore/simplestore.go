package simplestore

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/resources"
	"github.com/companyzero/bisonrelay/internal/jsonfile"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdscript"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/slog"
	"github.com/fsnotify/fsnotify"
	"github.com/pelletier/go-toml"
	"golang.org/x/sync/errgroup"
)

const (
	productsDir         = "products"
	cartsDir            = "carts"
	ordersDir           = "orders"
	pendingInvoicesDir  = "pendinginvoices"
	indexTmplFile       = "index.tmpl"
	prodTmplFile        = "product.tmpl"
	addToCartTmplFile   = "addtocart.tmpl"
	cartTmplFile        = "cart.tmpl"
	orderTmplFile       = "order.tmpl"
	ordersTmplFile      = "orders.tmpl"
	orderPlacedTmplFile = "orderplaced.tmpl"
	adminOrdersTmplFile = "admin_orders.tmpl"
	adminOrderTmplFile  = "admin_order.tmpl"
)

type PayType string

const (
	PayTypeOnChain PayType = "onchain"
	PayTypeLN      PayType = "ln"
)

// Config holds the configuration for a simple store.
type Config struct {
	Root          string
	Log           slog.Logger
	LiveReload    bool
	OrderPlaced   func(order *Order, msg string)
	StatusChanged func(order *Order, msg string)
	PayType       PayType
	Account       string
	ShipCharge    float64
	Client        *client.Client
	LNPayClient   *client.DcrlnPaymentClient

	ExchangeRateProvider func() float64
}

// Store is a simple store instance. A simple store can render a front page
// (index) and individual product pages.
type Store struct {
	cfg         Config
	c           *client.Client
	log         slog.Logger
	root        string
	lnpc        *client.DcrlnPaymentClient
	runCtx      context.Context
	runCancel   func()
	chainParams *chaincfg.Params

	mtx      sync.Mutex
	products map[string]*Product
	tmpl     *template.Template

	invoiceSettledChan  chan string
	invoiceCanceledChan chan string
	invoiceCreatedChan  chan *Order
}

// New creates a new simple store.
func New(cfg Config) (*Store, error) {
	log := slog.Disabled
	if cfg.Log != nil {
		log = cfg.Log
	}
	runCtx, runCancel := context.WithCancel(context.Background())

	s := &Store{
		cfg:       cfg,
		c:         cfg.Client,
		log:       log,
		root:      cfg.Root,
		products:  make(map[string]*Product),
		tmpl:      template.New("*root"),
		lnpc:      cfg.LNPayClient,
		runCtx:    runCtx,
		runCancel: runCancel,

		invoiceSettledChan:  make(chan string),
		invoiceCanceledChan: make(chan string),
		invoiceCreatedChan:  make(chan *Order),
	}

	if err := s.reloadStore(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) reloadStore() error {
	// Reset.
	products := make(map[string]*Product, len(s.products))
	tmpl := template.New("*root")

	// Parse templates.
	dirs := []string{s.root, filepath.Join(s.root, "static")}
	for _, dir := range dirs {
		filenames, err := filepath.Glob(filepath.Join(dir, "*.tmpl"))
		if err != nil {
			return err
		}
		for _, filename := range filenames {
			if filepath.Ext(filename) != ".tmpl" {
				continue
			}
			rawBytes, err := os.ReadFile(filename)
			if err != nil {
				return err
			}
			data := string(rawBytes)
			data = resources.ProcessEmbeds(data,
				s.root, s.log)

			s.log.Debugf("Reloading demplate %s (name %s)", filename, filepath.Base(filename))
			t := tmpl.New(filepath.Base(filename))
			_, err = t.Parse(data)
			if err != nil {
				return fmt.Errorf("unable to parse template %s: %v",
					filename, err)
			}
		}
	}

	// Load Products.
	prodDir := filepath.Join(s.root, productsDir)
	prodFiles, err := os.ReadDir(prodDir)
	if err != nil {
		return fmt.Errorf("unable to list product files: %v", err)
	}

	for _, prodFile := range prodFiles {
		if filepath.Ext(prodFile.Name()) != ".toml" {
			continue
		}
		fname := filepath.Join(prodDir, prodFile.Name())
		var prods productsFile
		f, err := os.Open(fname)
		if err != nil {
			return fmt.Errorf("unable to load product file %s: %v",
				fname, err)
		}
		dec := toml.NewDecoder(f)
		err = dec.Decode(&prods)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("unable to decode product file %s: %v",
				fname, err)
		}
		for _, prod := range prods.Products {
			if prod.Disabled {
				continue
			}

			if _, ok := products[prod.SKU]; ok {
				return fmt.Errorf("product with duplicated SKU %s in %s",
					prod.SKU, fname)
			}

			products[prod.SKU] = prod
		}
	}

	s.mtx.Lock()
	s.products = products
	s.tmpl = tmpl
	s.mtx.Unlock()

	return nil
}

func (s *Store) Fulfill(ctx context.Context, uid clientintf.UserID,
	request *rpc.RMFetchResource) (*rpc.RMFetchResourceReply, error) {

	// Admin handlers.
	if len(request.Path) > 0 && request.Path[0] == "admin" {
		if uid != s.c.PublicID() {
			return s.handleNotFound(ctx, uid, request)
		}
		switch {
		case pathEquals(request.Path, "admin"):
			return s.handleAdminIndex(ctx, uid, request)
		case pathEquals(request.Path, "admin", "orders"):
			return s.handleAdminOrders(ctx, uid, request)
		case pathHasPrefix(request.Path, "admin", "order"):
			return s.handleAdminViewOrder(ctx, uid, request)
		case pathHasPrefix(request.Path, "admin", "orderaddcomment"):
			return s.handleAdminAddOrderComment(ctx, uid, request)
		case pathHasPrefix(request.Path, "admin", "orderstatusto"):
			return s.handleAdminUpdateOrderStatus(ctx, uid, request)
		default:
			return s.handleNotFound(ctx, uid, request)
		}
	}

	switch {
	case len(request.Path) == 0 || request.Path[0] == "index.md":
		return s.handleIndex(ctx, uid, request)
	case len(request.Path) == 2 && request.Path[0] == "product":
		return s.handleProduct(ctx, uid, request)
	case pathEquals(request.Path, "addToCart"):
		return s.handleAddToCart(ctx, uid, request)
	case len(request.Path) == 1 && request.Path[0] == "clearCart":
		return s.handleClearCart(ctx, uid)
	case len(request.Path) == 1 && request.Path[0] == "cart":
		return s.handleCart(ctx, uid, request)
	case len(request.Path) == 1 && request.Path[0] == "placeOrder":
		return s.handlePlaceOrder(ctx, uid, request)
	case len(request.Path) == 1 && request.Path[0] == "orders":
		return s.handleOrders(ctx, uid, request)
	case len(request.Path) == 2 && request.Path[0] == "order":
		return s.handleOrderStatus(ctx, uid, request)
	case len(request.Path) == 2 && request.Path[0] == "orderaddcomment":
		return s.handleOrderAddComment(ctx, uid, request)
	case len(request.Path) == 2 && request.Path[0] == "static":
		return s.handleStaticRequest(ctx, uid, request)
	default:
		return s.handleNotFound(ctx, uid, request)
	}
}

func (s *Store) reloadFSWatchers(watcher *fsnotify.Watcher) {
	// We ignore watching errors here as these are not critical to the
	// operation of the store.

	prevWatches := watcher.WatchList()
	for _, w := range prevWatches {
		err := watcher.Remove(w)
		if err != nil {
			s.log.Warnf("Unable to remove previous watcher %s: %v",
				w, err)
		}
	}

	if err := watcher.Add(filepath.Join(s.root, productsDir)); err != nil {
		s.log.Warnf("Unable to watch products dir: %v", err)
	}

	if err := watcher.Add(filepath.Join(s.root)); err != nil {
		s.log.Warnf("Unable to watch root dir: %v", err)
	}
}

func (s *Store) runFSWatcher(ctx context.Context, watcher *fsnotify.Watcher) {
	s.reloadFSWatchers(watcher)

	// chanReload is used to debounce file events so that we only reload
	// once when multiple events happen in sequence.
	var chanReload <-chan time.Time

	s.log.Debugf("Starting FS watcher")
	for {
		select {
		case <-ctx.Done():
			return

		case <-chanReload:
			chanReload = nil
			err := s.reloadStore()
			if err != nil {
				s.log.Errorf("Unable to reload store: %v", err)
			} else {
				s.log.Infof("Reloaded store")
			}
			s.reloadFSWatchers(watcher)

		case event, ok := <-watcher.Events:
			if !ok {
				s.log.Warnf("watcher.Events not ok")
				return
			}
			s.log.Debugf("Watcher event: %s", event)
			chanReload = time.After(time.Millisecond * 100)

		case err, ok := <-watcher.Errors:
			if !ok {
				s.log.Warnf("watcher.Errors not ok")
				return
			}
			s.log.Debugf("Watcher error: %v", err)
		}
	}
}

// runLNInvoiceWatcher watches LN invoices and tells the main invoice watcher
// routine whenever one is settled.
func (s *Store) runLNInvoiceWatcher(ctx context.Context) error {
	stream, err := s.lnpc.LNRPC().SubscribeInvoices(ctx, &lnrpc.InvoiceSubscription{})
	if err != nil {
		return err
	}

	for {
		inv, err := stream.Recv()
		if err != nil {
			return err
		}

		if inv.State == lnrpc.Invoice_SETTLED {
			select {
			case s.invoiceSettledChan <- inv.PaymentRequest:
			case <-ctx.Done():
				return ctx.Err()
			}
		} else if inv.State == lnrpc.Invoice_CANCELED {
			select {
			case s.invoiceCanceledChan <- inv.PaymentRequest:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// runOnChainInvoiceWatcher watches for on-chain transactions that may complete
// orders.
func (s *Store) runOnChainInvoiceWatcher(ctx context.Context) error {
	// TODO: have some way to look for transactions upon restart.

	stream, err := s.lnpc.LNRPC().SubscribeTransactions(ctx, &lnrpc.GetTransactionsRequest{})
	if err != nil {
		return err
	}
	for {
		tx, err := stream.Recv()
		if err != nil {
			return err
		}

		// TODO: use different number of confirmations based on the
		// the amount.
		if tx.NumConfirmations < 1 {
			continue
		}

		msgTx := wire.NewMsgTx()
		if err := msgTx.Deserialize(hex.NewDecoder(bytes.NewBuffer([]byte(tx.RawTxHex)))); err != nil {
			s.log.Warnf("Unable to deserialize raw tx %s", tx.TxHash)
			continue
		}

		for _, out := range msgTx.TxOut {
			_, addrs := stdscript.ExtractAddrs(out.Version, out.PkScript, s.chainParams)
			if len(addrs) != 1 {
				// All addressses we create here are standard
				// P2PKH, so skip any that are not that.
				continue
			}

			discriminator := onChainInvoiceDiscriminator(addrs[0].String(), dcrutil.Amount(out.Value))
			select {
			case s.invoiceSettledChan <- discriminator:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// removePendingInvoice removes an order from the list of orders with pending
// invoice.
func (s *Store) removePendingInvoice(order *Order) {
	dir := filepath.Join(s.root, pendingInvoicesDir)
	fname := filepath.Join(dir, fmt.Sprintf("%s-%s", order.User, order.ID))
	err := jsonfile.RemoveIfExists(fname)
	if err != nil {
		s.log.Warnf("Unable to remove pending order %s: %v",
			fname, err)
	}
}

// invoiceSettled is called when an invoice for a given order was settled (paid)
// by the user.
func (s *Store) invoiceSettled(ctx context.Context, order *Order) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Remove pending invoice if exists.
	s.removePendingInvoice(order)

	// Mark order as paid. First, reload the full order from disk.
	orderDir := filepath.Join(s.root, ordersDir, order.User.String())
	orderFname := filepath.Join(orderDir, orderFnamePattern.FilenameFor(uint64(order.ID)))
	order = new(Order)
	if err := jsonfile.Read(orderFname, order); err != nil {
		s.log.Warnf("Unable to read order %s: %v", orderFname, err)
		return
	}

	// Now update status.
	order.Status = StatusPaid
	if err := jsonfile.Write(orderFname, order, s.log); err != nil {
		s.log.Warnf("Unable to write order %s: %v", orderFname, err)
		return
	}

	ru, err := s.c.UserByID(order.User)
	if err != nil {
		s.log.Warnf("Order #%d placed by unknown user %s",
			order.ID, order.User)
		return
	}

	s.log.Infof("Detected order %s/%s from user %s as paid",
		order.User.ShortLogID(), order.ID, strescape.Nick(ru.Nick()))

	// Finally, send a message to user acknowledging payment.
	var b strings.Builder
	wpm := func(f string, args ...interface{}) {
		b.WriteString(fmt.Sprintf(f, args...))
	}
	wpm("Your order %s/%s has been identified as paid",
		order.User.ShortLogID(), order.ID)

	// If the order has files attached to it, send them to the user.
	for _, item := range order.Cart.Items {
		fname := item.Product.SendFilename
		if item.Product.SendFilename == "" {
			continue
		}

		// Relative paths are set to be from the root of the simplestore.
		if !filepath.IsAbs(fname) {
			fname = filepath.Join(s.root, fname)
		}
		wpm("\nSending you the file %s included in your order",
			filepath.Base(fname))
		go func() {
			err := s.c.SendFile(order.User, fname)
			s.log.Errorf("Unable to send file %s to user %s due to order %s/%s: %v",
				fname, strescape.Nick(ru.Nick()),
				order.User.ShortLogID(), order.ID, err)
		}()
	}

	if s.cfg.StatusChanged != nil {
		s.cfg.StatusChanged(order, b.String())
	}
}

// invoiceExpired is called when the invoice of an order has expired.
func (s *Store) invoiceExpired(ctx context.Context, order *Order) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Remove pending invoice if exists.
	s.removePendingInvoice(order)

	// Mark order as paid. First, reload the full order from disk.
	orderDir := filepath.Join(s.root, ordersDir, order.User.String())
	orderFname := filepath.Join(orderDir, orderFnamePattern.FilenameFor(uint64(order.ID)))
	order = new(Order)
	if err := jsonfile.Read(orderFname, order); err != nil {
		s.log.Warnf("Unable to read order %s: %v", orderFname, err)
		return
	}

	// Now update status.
	order.Status = StatusPaid
	if err := jsonfile.Write(orderFname, order, s.log); err != nil {
		s.log.Warnf("Unable to write order %s: %v", orderFname, err)
		return
	}

	ru, err := s.c.UserByID(order.User)
	if err != nil {
		s.log.Warnf("Order #%d placed by unknown user %s",
			order.ID, order.User)
		return
	}

	s.log.Infof("Detected order %s/%s from user %s as expired",
		order.User.ShortLogID(), order.ID, strescape.Nick(ru.Nick()))

	// Finally, send a message to user noting the expiration.
	var b strings.Builder
	wpm := func(f string, args ...interface{}) {
		b.WriteString(fmt.Sprintf(f, args...))
	}
	wpm("Your order %s/%s has been identified as expired",
		order.User.ShortLogID(), order.ID)

	if s.cfg.StatusChanged != nil {
		s.cfg.StatusChanged(order, b.String())
	}
}

// runInvoiceWatcher is the main routine that handles changes to the status
// of invoices associated with orders.
func (s *Store) runInvoiceWatcher(ctx context.Context) error {
	// List orders with pending invoices.
	s.mtx.Lock()
	dirPending := filepath.Join(s.root, pendingInvoicesDir)
	entries, err := os.ReadDir(dirPending)
	if err != nil && !os.IsNotExist(err) {
		s.mtx.Unlock()
		return err
	}

	// Create map of raw invoice to pending id.
	invoices := make(map[string]*Order, len(entries))

	// Load list of pending orders. The names in the pending invoices
	// dir is "<uid>-<order_id>".
	nameRegexp := regexp.MustCompile(`([0-9a-fA-F]{64})-([0-9]*)`)
	for _, entry := range entries {
		name := entry.Name()
		matches := nameRegexp.FindStringSubmatch(name)
		if len(matches) != 3 {
			continue
		}
		var uid clientintf.UserID
		if err := uid.FromString(matches[1]); err != nil {
			continue
		}
		var oid OrderID
		if err := oid.FromString(matches[2]); err != nil {
			continue
		}
		order := new(Order)
		fname := filepath.Join(s.root, ordersDir, uid.String(),
			orderFnamePattern.FilenameFor(uint64(oid)))
		if err := jsonfile.Read(fname, order); err != nil {
			s.log.Warnf("Unable to load order %s: %v", fname, err)
			continue
		}
		if order.Invoice == "" || order.ExpiresTS.Before(time.Now()) {
			go s.invoiceExpired(ctx, order)
			continue
		}
		if order.Status != StatusPlaced {
			s.removePendingInvoice(order)
			continue
		}
		invoices[order.invoiceDiscriminator()] = order
	}
	s.mtx.Unlock()

	// Timer that is triggered on the next time one of the invoices needs
	// to be timed out.
	nextExpiresTimer := time.NewTimer(time.Duration(math.MaxInt64))
	nextExpiresTimer.Stop()
	resetNextExpiresTimer := func() {
		var nextExpiresTime time.Time
		for _, order := range invoices {
			if nextExpiresTime.IsZero() || order.ExpiresTS.Before(nextExpiresTime) {
				nextExpiresTime = order.ExpiresTS
			}
		}
		if nextExpiresTime.IsZero() {
			return
		}
		nextExpiresTimer.Reset(time.Until(nextExpiresTime))
	}
	resetNextExpiresTimer()

	// Main loop: handle the outcome of invoices.
	for {
		select {
		case order := <-s.invoiceCreatedChan:
			invoices[order.invoiceDiscriminator()] = order

		case inv := <-s.invoiceSettledChan:
			if order := invoices[inv]; order != nil {
				delete(invoices, inv)
				go s.invoiceSettled(ctx, order)
			}

		case inv := <-s.invoiceCanceledChan:
			delete(invoices, inv)

		case <-nextExpiresTimer.C:
			now := time.Now()
			for _, order := range invoices {
				if !order.ExpiresTS.Before(now) {
					continue
				}
				delete(invoices, order.invoiceDiscriminator())
				go s.invoiceExpired(ctx, order)
			}
			resetNextExpiresTimer()

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Run the simple store functions.
func (s *Store) Run(ctx context.Context) error {
	chainParams, err := s.lnpc.ChainParams(ctx)
	if err != nil {
		return err
	}
	s.chainParams = chainParams

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-gctx.Done()
		s.runCancel()
		return gctx.Err()
	})

	if s.cfg.LiveReload {
		g.Go(func() error {
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return fmt.Errorf("unable to start filesystem watcher: %s", err)
			}

			s.runFSWatcher(gctx, watcher)
			return watcher.Close()
		})
	}

	g.Go(func() error { return s.runLNInvoiceWatcher(ctx) })
	g.Go(func() error { return s.runOnChainInvoiceWatcher(ctx) })
	g.Go(func() error { return s.runInvoiceWatcher(ctx) })

	return g.Wait()
}
