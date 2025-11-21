package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// randRead is a wrapper around crypto/rand.Read
func randRead(b []byte) (int, error) {
	return rand.Read(b)
}

// generateSalt creates a random salt for password hashing
func generateSalt(length int) (string, error) {
	salt := make([]byte, length)
	if _, err := randRead(salt); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}

// hashPassword hashes a password using Argon2id with a randomly generated salt
func hashPassword(password string) (string, error) {
	// Generate a random salt (16 bytes = 128 bits)
	salt, err := generateSalt(16)
	if err != nil {
		return "", err
	}

	// Decode the salt for hashing
	saltBytes, _ := base64.StdEncoding.DecodeString(salt)

	// Argon2id parameters (balanced between security and performance)
	hash := argon2.IDKey([]byte(password), saltBytes, 1, 64*1024, 4, 32)

	// Encode hash to base64
	hashStr := base64.StdEncoding.EncodeToString(hash)

	// Return combined salt:hash format for storage
	return salt + ":" + hashStr, nil
}

// verifyPassword compares a password with a stored hash
func verifyPassword(password, storedHash string) bool {
	// Split the stored hash into salt and hash
	parts := strings.Split(storedHash, ":")
	if len(parts) != 2 {
		return false
	}

	salt := parts[0]
	hashStr := parts[1]

	// Decode the salt
	saltBytes, err := base64.StdEncoding.DecodeString(salt)
	if err != nil {
		return false
	}

	// Hash the provided password with the same salt
	hash := argon2.IDKey([]byte(password), saltBytes, 1, 64*1024, 4, 32)
	hashStr2 := base64.StdEncoding.EncodeToString(hash)

	// Compare hashes
	return hashStr == hashStr2
}

// initDB initializes the database schema
func initDB(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		litecoin_address TEXT UNIQUE,
		kernelcoin_address TEXT UNIQUE,
		litecoin_receive_address TEXT,
		kernelcoin_receive_address TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS balances (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER UNIQUE NOT NULL,
		litecoin REAL DEFAULT 0,
		kernelcoin REAL DEFAULT 0,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS trades (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		seller_id INTEGER NOT NULL,
		coin_selling TEXT NOT NULL,
		amount_selling REAL NOT NULL,
		coin_buying TEXT NOT NULL,
		amount_buying REAL NOT NULL,
		price_per_unit REAL NOT NULL,
		status TEXT DEFAULT 'open',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		cancelled_at TIMESTAMP,
		FOREIGN KEY(seller_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		coin TEXT NOT NULL,
		amount REAL NOT NULL,
		type TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		tx_hash TEXT,
		related_trade INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS trade_completions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		trade_id INTEGER NOT NULL,
		buyer_id INTEGER NOT NULL,
		quantity REAL NOT NULL,
		completed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(trade_id) REFERENCES trades(id),
		FOREIGN KEY(buyer_id) REFERENCES users(id)
	);

	CREATE INDEX IF NOT EXISTS idx_trades_seller ON trades(seller_id);
	CREATE INDEX IF NOT EXISTS idx_trades_status ON trades(status);
	CREATE INDEX IF NOT EXISTS idx_transactions_user ON transactions(user_id);
	CREATE INDEX IF NOT EXISTS idx_balances_user ON balances(user_id);
	`

	_, err := db.Exec(schema)
	return err
}

// preseedDB populates the database with initial data
func preseedDB(db *sql.DB) error {
	// Generate password hashes for known passwords
	mikeHash, err := hashPassword("pass")
	if err != nil {
		return err
	}
	bobHash, err := hashPassword("pass")
	if err != nil {
		return err
	}

	// Insert users
	_, err = db.Exec(`
		INSERT OR REPLACE INTO users
		(id, username, password_hash, created_at)
		VALUES 
		(1, 'mike', ?, '2025-11-19 23:48:07'),
		(2, 'bob', ?, '2025-11-19 23:48:12')
	`, mikeHash, bobHash)
	if err != nil {
		return err
	}

	// Insert balances
	_, err = db.Exec(`
		INSERT OR REPLACE INTO balances (id, user_id, litecoin, kernelcoin)
		VALUES 
		(1, 1, 1000.0, 1000.0),
		(2, 2, 1000.0, 1000.0)
	`)
	if err != nil {
		return err
	}

	// Update sqlite_sequence
	_, err = db.Exec(`
		INSERT OR REPLACE INTO sqlite_sequence (name, seq)
		VALUES 
		('users', 2),
		('balances', 2)
	`)

	return err
}

// getUserByUsername retrieves a user from the database by username
func (s *Server) getUserByUsername(username string) (*User, error) {
	var user User
	err := s.db.QueryRow(`SELECT id, username, password_hash FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.PasswordHash)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// createUser inserts a new user into the database
func (s *Server) createUser(username, passwordHash string) (int, error) {
	result, err := s.db.Exec(
		`INSERT INTO users (username, password_hash) 
		 VALUES (?, ?)`,
		username, passwordHash,
	)

	if err != nil {
		return 0, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Create balance entry
	_, err = s.db.Exec(`INSERT INTO balances (user_id, litecoin, kernelcoin) VALUES (?, 0, 0)`, userID)
	if err != nil {
		return 0, err
	}

	return int(userID), nil
}

// getUserBalance retrieves a user's balance
func (s *Server) getUserBalance(userID int) (*Balance, error) {
	var balance Balance
	balance.UserID = userID
	err := s.db.QueryRow(`SELECT litecoin, kernelcoin FROM balances WHERE user_id = ?`, userID).
		Scan(&balance.Litecoin, &balance.Kernelcoin)
	if err != nil {
		return nil, err
	}
	return &balance, nil
}

// getAggregateBalance retrieves total balances across all users
func (s *Server) getAggregateBalance() (ltc, kcn float64, userCount int, err error) {
	err = s.db.QueryRow(`SELECT SUM(litecoin), SUM(kernelcoin) FROM balances`).
		Scan(&ltc, &kcn)
	if err != nil {
		return 0, 0, 0, err
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount)
	return ltc, kcn, userCount, err
}

// getAllUsers retrieves all users with their balances
func (s *Server) getAllUsers() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.litecoin_address, u.kernelcoin_address, 
		       COALESCE(b.litecoin, 0), COALESCE(b.kernelcoin, 0)
		FROM users u
		LEFT JOIN balances b ON u.id = b.user_id
		ORDER BY u.id
	`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id int
		var username, ltcAddr, kcnAddr string
		var ltcBalance, kcnBalance float64

		err := rows.Scan(&id, &username, &ltcAddr, &kcnAddr, &ltcBalance, &kcnBalance)
		if err != nil {
			continue
		}

		// Calculate reserved amounts for this user
		var ltcReserved, kcnReserved float64
		reservedRows, err := s.db.Query(`
			SELECT coin_selling, amount_selling FROM trades 
			WHERE seller_id = ? AND status = 'open'
		`, id)
		if err == nil {
			defer reservedRows.Close()
			for reservedRows.Next() {
				var coin string
				var amount float64
				if reservedRows.Scan(&coin, &amount) == nil {
					if coin == "litecoin" {
						ltcReserved += amount
					} else {
						kcnReserved += amount
					}
				}
			}
		}

		user := map[string]interface{}{
			"id":                 id,
			"username":           username,
			"litecoin_address":   ltcAddr,
			"kernelcoin_address": kcnAddr,
			"litecoin_balance":   ltcBalance,
			"kernelcoin_balance": kcnBalance,
			"ltc_reserved":       ltcReserved,
			"kcn_reserved":       kcnReserved,
		}
		users = append(users, user)
	}

	return users, nil
}

// createTrade creates a new trade and reserves coins
func (s *Server) createTrade(sellerID int, coinSelling string, amountSelling float64,
	coinBuying string, amountBuying float64) (int64, error) {

	// Calculate price per unit correctly based on what's being sold
	var pricePerUnit float64
	if coinSelling == "kernelcoin" {
		// Selling KCN for LTC: price = LTC per KCN
		pricePerUnit = amountBuying / amountSelling
	} else {
		// Selling LTC for KCN: price = LTC per KCN
		pricePerUnit = amountSelling / amountBuying
	}

	result, err := s.db.Exec(`
		INSERT INTO trades (seller_id, coin_selling, amount_selling, coin_buying, amount_buying, price_per_unit)
		VALUES (?, ?, ?, ?, ?, ?)
	`, sellerID, coinSelling, amountSelling, coinBuying, amountBuying, pricePerUnit)

	if err != nil {
		return 0, err
	}

	tradeID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Reserve coins
	coinCol := coinSelling
	_, err = s.db.Exec(fmt.Sprintf(`UPDATE balances SET %s = %s - ? WHERE user_id = ?`, coinCol, coinCol),
		amountSelling, sellerID)

	return tradeID, err
}

// getOpenTrades retrieves all open trades
func (s *Server) getOpenTrades() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT id, seller_id, coin_selling, amount_selling, coin_buying, amount_buying, 
		       price_per_unit, status, created_at
		FROM trades
		WHERE status = 'open'
		ORDER BY created_at DESC
	`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []map[string]interface{}
	for rows.Next() {
		var id, sellerID int
		var coinSelling, coinBuying, status string
		var amountSelling, amountBuying, pricePerUnit float64
		var createdAt string

		err := rows.Scan(&id, &sellerID, &coinSelling, &amountSelling, &coinBuying, &amountBuying,
			&pricePerUnit, &status, &createdAt)
		if err != nil {
			continue
		}

		// Get seller name
		var sellerName string
		s.db.QueryRow(`SELECT username FROM users WHERE id = ?`, sellerID).Scan(&sellerName)

		trade := map[string]interface{}{
			"id":             id,
			"seller_id":      sellerID,
			"seller_name":    sellerName,
			"coin_selling":   coinSelling,
			"amount_selling": amountSelling,
			"coin_buying":    coinBuying,
			"amount_buying":  amountBuying,
			"price_per_unit": pricePerUnit,
			"price_ltc":      amountBuying,
			"status":         status,
			"created_at":     createdAt,
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// getUserTrades retrieves trades for a specific user (both as seller and buyer)
func (s *Server) getUserTrades(userID int) ([]map[string]interface{}, error) {
	// Get trades where user was the seller
	rows, err := s.db.Query(`
		SELECT t.id, t.seller_id, t.coin_selling, t.amount_selling, t.coin_buying, t.amount_buying, 
		       t.price_per_unit, t.status, t.created_at, NULL as counterparty
		FROM trades t
		WHERE t.seller_id = ?
		UNION ALL
		SELECT t.id, t.seller_id, t.coin_buying as coin_selling, tc.quantity as amount_selling, 
		       t.coin_selling as coin_buying, (tc.quantity * t.price_per_unit) as amount_buying,
		       t.price_per_unit, 'completed' as status, tc.completed_at as created_at,
		       u.username as counterparty
		FROM trade_completions tc
		JOIN trades t ON tc.trade_id = t.id
		JOIN users u ON t.seller_id = u.id
		WHERE tc.buyer_id = ?
		ORDER BY created_at DESC
	`, userID, userID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []map[string]interface{}
	for rows.Next() {
		var id, sellerID int
		var coinSelling, coinBuying, status, createdAt string
		var amountSelling, amountBuying, pricePerUnit float64
		var counterparty sql.NullString

		err := rows.Scan(&id, &sellerID, &coinSelling, &amountSelling, &coinBuying, &amountBuying,
			&pricePerUnit, &status, &createdAt, &counterparty)
		if err != nil {
			continue
		}

		trade := map[string]interface{}{
			"id":             id,
			"coin_selling":   coinSelling,
			"amount_selling": amountSelling,
			"coin_buying":    coinBuying,
			"amount_buying":  amountBuying,
			"price_per_unit": pricePerUnit,
			"price_ltc":      amountBuying,
			"status":         status,
			"created_at":     createdAt,
		}
		
		if counterparty.Valid {
			trade["counterparty"] = counterparty.String
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

// getTrade retrieves a specific trade
func (s *Server) getTrade(tradeID int) (map[string]interface{}, error) {
	var id, sellerID int
	var coinSelling, coinBuying, status string
	var amountSelling, amountBuying, pricePerUnit float64

	err := s.db.QueryRow(`
		SELECT id, seller_id, coin_selling, amount_selling, coin_buying, amount_buying, price_per_unit, status
		FROM trades WHERE id = ?
	`, tradeID).Scan(&id, &sellerID, &coinSelling, &amountSelling, &coinBuying, &amountBuying, &pricePerUnit, &status)

	if err != nil {
		return nil, err
	}

	trade := map[string]interface{}{
		"id":             id,
		"seller_id":      sellerID,
		"coin_selling":   coinSelling,
		"amount_selling": amountSelling,
		"coin_buying":    coinBuying,
		"amount_buying":  amountBuying,
		"price_per_unit": pricePerUnit,
		"status":         status,
	}

	return trade, nil
}

// executeTrade executes a trade between buyer and seller
func (s *Server) executeTrade(tradeID int, buyerID int, quantity float64) error {
	trade, err := s.getTrade(tradeID)
	if err != nil {
		return err
	}

	sellerID := trade["seller_id"].(int)
	coinSelling := trade["coin_selling"].(string)
	coinBuying := trade["coin_buying"].(string)
	pricePerUnit := trade["price_per_unit"].(float64)
	amountBuying := trade["amount_buying"].(float64)

	// The quantity parameter represents how much KCN is being traded
	// Determine what the buyer is giving and receiving based on the trade structure
	var buyerGives, buyerReceives string
	var buyerGivesAmount, buyerReceivesAmount float64

	if coinBuying == "kernelcoin" {
		// This is a BUY order (seller wants KCN, offers LTC)
		// Buyer gives KCN, receives LTC
		buyerGives = "kernelcoin"
		buyerReceives = "litecoin"
		buyerGivesAmount = quantity
		buyerReceivesAmount = quantity * pricePerUnit
	} else {
		// This is a SELL order (seller offers KCN, wants LTC)
		// Buyer gives LTC, receives KCN
		buyerGives = "litecoin"
		buyerReceives = "kernelcoin"
		buyerGivesAmount = quantity * pricePerUnit
		buyerReceivesAmount = quantity
	}

	// Check balances
	buyerBal, _ := s.getUserBalance(buyerID)
	sellerBal, _ := s.getUserBalance(sellerID)

	// Check buyer has enough of what they're giving
	var buyerBalance float64
	if buyerGives == "litecoin" {
		buyerBalance = buyerBal.Litecoin
	} else {
		buyerBalance = buyerBal.Kernelcoin
	}

	if buyerBalance < buyerGivesAmount {
		return fmt.Errorf("insufficient balance")
	}

	// Check seller has enough reserved coins
	var sellerCoinBalance float64
	if coinSelling == "litecoin" {
		sellerCoinBalance = sellerBal.Litecoin
	} else {
		sellerCoinBalance = sellerBal.Kernelcoin
	}

	if sellerCoinBalance < buyerReceivesAmount {
		return fmt.Errorf("trade amount unavailable")
	}

	// Execute the trade:
	// 1. Buyer loses what they're giving
	_, err = s.db.Exec(fmt.Sprintf(`UPDATE balances SET %s = %s - ? WHERE user_id = ?`, buyerGives, buyerGives),
		buyerGivesAmount, buyerID)
	if err != nil {
		return err
	}

	// 2. Buyer receives what they're getting
	_, err = s.db.Exec(fmt.Sprintf(`UPDATE balances SET %s = %s + ? WHERE user_id = ?`, buyerReceives, buyerReceives),
		buyerReceivesAmount, buyerID)
	if err != nil {
		return err
	}

	// 3. Seller receives what buyer gave
	_, err = s.db.Exec(fmt.Sprintf(`UPDATE balances SET %s = %s + ? WHERE user_id = ?`, buyerGives, buyerGives),
		buyerGivesAmount, sellerID)
	if err != nil {
		return err
	}

	// Note: Seller's coinSelling was already deducted when the trade was created (reserved)

	_, err = s.db.Exec(`INSERT INTO trade_completions (trade_id, buyer_id, quantity) VALUES (?, ?, ?)`,
		tradeID, buyerID, quantity)

	// Check if trade is fully completed
	if quantity >= amountBuying {
		_, err = s.db.Exec(`UPDATE trades SET status = 'completed' WHERE id = ?`, tradeID)
	}

	return err
}

// cancelTrade cancels a trade and returns coins to seller
func (s *Server) cancelTrade(tradeID int, userID int) error {
	trade, err := s.getTrade(tradeID)
	if err != nil {
		return err
	}

	sellerID := trade["seller_id"].(int)
	if sellerID != userID {
		return fmt.Errorf("cannot cancel trade you don't own")
	}

	coinSelling := trade["coin_selling"].(string)
	amountSelling := trade["amount_selling"].(float64)

	_, err = s.db.Exec(
		fmt.Sprintf(`UPDATE balances SET %s = %s + ? WHERE user_id = ?`, coinSelling, coinSelling),
		amountSelling, userID,
	)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE trades SET status = 'cancelled', cancelled_at = CURRENT_TIMESTAMP WHERE id = ?`, tradeID)
	return err
}

// getPriceStats calculates KCN price statistics
func (s *Server) getPriceStats() (avgPrice, minPrice float64, err error) {
	var avgKCNPrice, minKCNPrice sql.NullFloat64

	err = s.db.QueryRow(`
		SELECT AVG(amount_buying / amount_selling), MIN(amount_buying / amount_selling) FROM trades 
		WHERE status = 'open' AND coin_selling = 'kernelcoin' AND coin_buying = 'litecoin'
	`).Scan(&avgKCNPrice, &minKCNPrice)

	if err != nil {
		return 0, 0, err
	}

	avgPrice = 1.0
	if avgKCNPrice.Valid {
		avgPrice = avgKCNPrice.Float64
	}

	minPrice = 1.0
	if minKCNPrice.Valid {
		minPrice = minKCNPrice.Float64
	}

	return avgPrice, minPrice, nil
}
