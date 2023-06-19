package simplestore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"text/template"
	"time"

	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/resources"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/slog"
	"github.com/fsnotify/fsnotify"
	"github.com/pelletier/go-toml"
	"golang.org/x/sync/errgroup"
)

const (
	productsDir         = "products"
	cartsDir            = "carts"
	ordersDir           = "orders"
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
	PayTypeLN      PayType = "LN"
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

	ExchangeRateProvider func() float64
}

// Store is a simple store instance. A simple store can render a front page
// (index) and individual product pages.
type Store struct {
	cfg  Config
	c    *client.Client
	log  slog.Logger
	root string
	lnpc *client.DcrlnPaymentClient

	mtx      sync.Mutex
	products map[string]*Product
	tmpl     *template.Template
}

// New creates a new simple store.
func New(cfg Config) (*Store, error) {
	log := slog.Disabled
	if cfg.Log != nil {
		log = cfg.Log
	}

	s := &Store{
		cfg:      cfg,
		c:        cfg.Client,
		log:      log,
		root:     cfg.Root,
		products: make(map[string]*Product),
		tmpl:     template.New("*root"),
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
	filenames, err := filepath.Glob(filepath.Join(s.root, "*.tmpl"))
	if err != nil {
		return err
	}
	for _, filename := range filenames {
		rawBytes, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		data := string(rawBytes)
		data = resources.ProcessEmbeds(data,
			s.root, s.log)

		t := tmpl.New(filepath.Base(filename))
		_, err = t.Parse(data)
		if err != nil {
			return fmt.Errorf("unable to parse template %s: %v",
				filename, err)
		}
	}

	// Load Products.
	prodDir := filepath.Join(s.root, productsDir)
	prodFiles, err := os.ReadDir(prodDir)
	if err != nil {
		return fmt.Errorf("unable to list product files: %v", err)
	}

	for _, prodFile := range prodFiles {
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

// Run the simple store functions.
func (s *Store) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

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
	return g.Wait()
}
