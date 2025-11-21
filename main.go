package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Session represents a user session
type Session struct {
	UserID   int
	Username string
	Expiry   time.Time
}

// Server is the main application server
type Server struct {
	db                  *sql.DB
	mu                  sync.RWMutex
	sessions            map[string]*Session
	captchaService      *CaptchaService
	kernelcoinRPCUser   string
	kernelcoinRPCPass   string
	kernelcoinRPCHost   string
	kernelcoinRPCPort   string
	kernelcoinRPCClient *KernelcoinRPCClient
	electrumClient      *ElectrumClient
	electrumBinary      string
	ltcWithdrawFee      float64
	noWallets           bool
	ltcPriceCache       float64
	ltcPriceCacheExpiry time.Time
}

// User represents a user account
type User struct {
	ID              int
	Username        string
	PasswordHash    string
	LitecoinAddress string
	KernelcoinAddr  string
}

// Trade represents a trade order
type Trade struct {
	ID            int
	SellerID      int
	SellerName    string
	CoinSelling   string
	AmountSelling float64
	CoinBuying    string
	AmountBuying  float64
	PricePerUnit  float64
	CreatedAt     time.Time
	Status        string // "open", "completed", "cancelled"
}

// Balance represents user balances
type Balance struct {
	UserID     int
	Litecoin   float64
	Kernelcoin float64
}

// Transaction represents a transaction
type Transaction struct {
	ID           int
	UserID       int
	Coin         string
	Amount       float64
	Type         string // "deposit", "withdraw", "trade"
	Status       string // "pending", "confirmed"
	TxHash       string
	RelatedTrade *int
	CreatedAt    time.Time
}

func main() {
	var (
		port              = flag.String("port", "8080", "Server port")
		dbPath            = flag.String("db", "exchange.db", "Database path")
		electrumBinary    = flag.String("electrum-binary", "/Applications/Electrum-LTC.app/Contents/MacOS/run_electrum", "Path to Electrum binary")
		ltcWithdrawFee    = flag.Float64("ltc-withdraw-fee", 0.0003, "Litecoin withdrawal fee")
		kernelcoinRPCUser = flag.String("kcn-rpc-user", "mike", "Kernelcoin RPC user")
		kernelcoinRPCPass = flag.String("kcn-rpc-pass", "x", "Kernelcoin RPC password")
		kernelcoinRPCHost = flag.String("kcn-rpc-host", "127.0.0.1", "Kernelcoin RPC host")
		kernelcoinRPCPort = flag.String("kcn-rpc-port", "9332", "Kernelcoin RPC port")
		noWallets         = flag.Bool("no-wallets", false, "Disable wallet integration and use fallback behavior")
		preseed           = flag.Bool("preseed", false, "Preseed database with test users")
	)

	flag.Parse()

	// Create database directory
	dbDir := filepath.Dir(*dbPath)
	if dbDir != "." && dbDir != "" {
		os.MkdirAll(dbDir, 0755)
	}

	// Open database
	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize database schema
	if err := initDB(db); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Preseed database with initial data if flag is provided
	if *preseed {
		if err := preseedDB(db); err != nil {
			log.Fatalf("Failed to preseed database: %v", err)
		}
	}

	// Create kernelcoin RPC client
	kernelcoinRPCURL := fmt.Sprintf("http://%s:%s", *kernelcoinRPCHost, *kernelcoinRPCPort)
	kernelcoinRPCClient := NewKernelcoinRPCClient(kernelcoinRPCURL, *kernelcoinRPCUser, *kernelcoinRPCPass)

	// Create Electrum client
	electrumClient := NewElectrumClient(*electrumBinary, *ltcWithdrawFee)

	// Create server instance
	server := &Server{
		db:                  db,
		sessions:            make(map[string]*Session),
		captchaService:      NewCaptchaService(),
		kernelcoinRPCUser:   *kernelcoinRPCUser,
		kernelcoinRPCPass:   *kernelcoinRPCPass,
		kernelcoinRPCHost:   *kernelcoinRPCHost,
		kernelcoinRPCPort:   *kernelcoinRPCPort,
		kernelcoinRPCClient: kernelcoinRPCClient,
		electrumClient:      electrumClient,
		electrumBinary:      *electrumBinary,
		ltcWithdrawFee:      *ltcWithdrawFee,
		noWallets:           *noWallets,
	}

	// Register all routes
	server.RegisterRoutes()

	// Start server
	log.Printf("Exchange server starting on http://127.0.0.1:%s", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
