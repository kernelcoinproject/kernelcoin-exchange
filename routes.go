package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// generateSessionToken creates a random session token
func generateSessionToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(token), nil
}

// isValidAlphanumeric checks if string contains only alphanumeric characters
func isValidAlphanumeric(s string) bool {
	matched, _ := regexp.MatchString("^[a-zA-Z0-9]+$", s)
	return matched
}

// getSession retrieves a session from a request
func (s *Server) getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}

	s.mu.RLock()
	session, ok := s.sessions[cookie.Value]
	s.mu.RUnlock()

	if !ok || session.Expiry.Before(time.Now()) {
		return nil
	}

	return session
}

// RegisterRoutes sets up all HTTP routes
func (s *Server) RegisterRoutes() {
	http.HandleFunc("/", s.serveHTML)
	http.HandleFunc("/api/captcha/generate", s.handleGenerateCaptcha)
	http.HandleFunc("/api/register", s.handleRegister)
	http.HandleFunc("/api/login", s.handleLogin)
	http.HandleFunc("/api/logout", s.handleLogout)
	http.HandleFunc("/api/session", s.handleCheckSession)
	http.HandleFunc("/api/balance", s.handleGetBalance)
	http.HandleFunc("/api/escrow", s.handleGetEscrow)
	http.HandleFunc("/api/admin", s.handleGetAdminData)
	http.HandleFunc("/api/trade/create", s.handleCreateTrade)
	http.HandleFunc("/api/trade/list", s.handleListTrades)
	http.HandleFunc("/api/trade/my-trades", s.handleGetUserTrades)
	http.HandleFunc("/api/trade/execute", s.handleExecuteTrade)
	http.HandleFunc("/api/trade/cancel", s.handleCancelTrade)
	http.HandleFunc("/api/price-stats", s.handleGetPriceStats)
	http.HandleFunc("/api/ltc-price", s.handleGetLtcPrice)
	http.HandleFunc("/api/user", s.handleGetUser)
	http.HandleFunc("/api/withdraw", s.handleWithdraw)
	http.HandleFunc("/api/transactions", s.handleGetTransactions)
	http.HandleFunc("/api/admin/stats", s.handleGetAdminStats)
	http.HandleFunc("/api/change-password", s.handleChangePassword)
	http.HandleFunc("/api/update-addresses", s.handleUpdateAddresses)
	http.HandleFunc("/api/generate-receive-address", s.handleGenerateReceiveAddress)
	http.HandleFunc("/api/check-confirmations", s.handleCheckConfirmations)
	http.HandleFunc("/api/withdraw-fee", s.handleGetWithdrawFee)
}

// handleGenerateCaptcha generates a new captcha
func (s *Server) handleGenerateCaptcha(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	captchaResp, err := s.captchaService.GenerateCaptcha()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate captcha"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(captchaResp)
}

// handleRegister handles user registration
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		CaptchaID string `json:"captcha_id"`
		CaptchaX  int    `json:"captcha_x"`
		CaptchaY  int    `json:"captcha_y"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// Validate captcha first
	log.Printf("[REGISTER] Received captcha validation request - ID: %s, Position: (%d, %d)", req.CaptchaID, req.CaptchaX, req.CaptchaY)
	if !s.captchaService.ValidateCaptcha(req.CaptchaID, req.CaptchaX, req.CaptchaY) {
		log.Printf("[REGISTER] Captcha validation failed for ID: %s", req.CaptchaID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid captcha"})
		return
	}
	log.Printf("[REGISTER] Captcha validation successful for ID: %s", req.CaptchaID)

	// Validate inputs
	if len(req.Username) == 0 || len(req.Password) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Username and password required"})
		return
	}

	if len(req.Password) > 64 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Password must be less than 64 characters"})
		return
	}

	// Validate alphanumeric and length
	if !isValidAlphanumeric(req.Username) || len(req.Username) > 64 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Username must be alphanumeric and less than 64 characters"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Password hashing failed"})
		return
	}

	_, err = s.createUser(req.Username, passwordHash)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Username already exists"})
		} else {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Registration failed"})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "User registered successfully",
	})
}

// handleLogin handles user login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// No captcha validation for login

	s.mu.Lock()
	defer s.mu.Unlock()

	user, err := s.getUserByUsername(req.Username)
	if err != nil || !verifyPassword(req.Password, user.PasswordHash) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}

	// Create session
	token, _ := generateSessionToken()
	s.sessions[token] = &Session{
		UserID:   user.ID,
		Username: user.Username,
		Expiry:   time.Now().Add(24 * time.Hour),
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"username": user.Username,
		"user_id":  user.ID,
	})
}

// handleLogout handles user logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, _ := r.Cookie("session")
	if cookie != nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		s.mu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// handleCheckSession checks if a user has a valid session
func (s *Server) handleCheckSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := s.getSession(r)
	w.Header().Set("Content-Type", "application/json")

	if session == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"username": session.Username,
		"user_id":  session.UserID,
	})
}

// handleGetBalance gets a user's balance including reserved amounts
func (s *Server) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	balance, err := s.getUserBalance(session.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Balance not found"})
		return
	}

	// Calculate reserved amounts from open trades
	var ltcReserved, kcnReserved float64
	rows, err := s.db.Query(`
		SELECT coin_selling, amount_selling FROM trades 
		WHERE seller_id = ? AND status = 'open'
	`, session.UserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var coin string
			var amount float64
			if rows.Scan(&coin, &amount) == nil {
				if coin == "litecoin" {
					ltcReserved += amount
				} else {
					kcnReserved += amount
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]float64{
		"litecoin":     balance.Litecoin,
		"kernelcoin":   balance.Kernelcoin,
		"ltc_reserved":  ltcReserved,
		"kcn_reserved":  kcnReserved,
	})
}

// handleGetEscrow gets aggregate escrow data
func (s *Server) handleGetEscrow(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalLTC, totalKCN, userCount, err := s.getAggregateBalance()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch escrow data"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_litecoin":   totalLTC,
		"total_kernelcoin": totalKCN,
		"total_users":      userCount,
	})
}

// handleGetAdminData gets admin panel data
func (s *Server) handleGetAdminData(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil || session.Username != "admin" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	users, err := s.getAllUsers()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch users"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": users})
}

// handleCreateTrade creates a new trade
func (s *Server) handleCreateTrade(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CoinSelling   string  `json:"coin_selling"`
		AmountSelling float64 `json:"amount_selling"`
		CoinBuying    string  `json:"coin_buying"`
		AmountBuying  float64 `json:"amount_buying"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check active trades limit
	var activeTradesCount int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM trades WHERE seller_id = ? AND status = 'open'`, session.UserID).Scan(&activeTradesCount)
	if err == nil && activeTradesCount >= 10 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Maximum of 10 active trades allowed per account"})
		return
	}

	balance, err := s.getUserBalance(session.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "User balance not found"})
		return
	}

	var userBalance float64
	if req.CoinSelling == "litecoin" {
		userBalance = balance.Litecoin
	} else {
		userBalance = balance.Kernelcoin
	}

	if userBalance < req.AmountSelling {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Insufficient balance"})
		return
	}

	tradeID, err := s.createTrade(session.UserID, req.CoinSelling, req.AmountSelling,
		req.CoinBuying, req.AmountBuying)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create trade"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"trade_id": tradeID,
	})
}

// handleListTrades lists all open trades
func (s *Server) handleListTrades(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trades, err := s.getOpenTrades()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch trades"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"trades": trades})
}

// handleGetUserTrades gets a user's trades
func (s *Server) handleGetUserTrades(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	trades, err := s.getUserTrades(session.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch trades"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"trades": trades})
}

// handleExecuteTrade executes a trade
func (s *Server) handleExecuteTrade(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		TradeID  int     `json:"trade_id"`
		Quantity float64 `json:"quantity"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.executeTrade(req.TradeID, session.UserID, req.Quantity)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// handleCancelTrade cancels a trade
func (s *Server) handleCancelTrade(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		TradeID int `json:"trade_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.cancelTrade(req.TradeID, session.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Trade cancelled",
	})
}

// handleGetPriceStats gets price statistics
func (s *Server) handleGetPriceStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	avgPrice, minPrice, err := s.getPriceStats()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch price stats"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"kernelcoin_avg_price": avgPrice,
		"kernelcoin_min_price": minPrice,
	})
}

// handleGetLtcPrice gets the current LTC price
func (s *Server) handleGetLtcPrice(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	// Check if cache is still valid
	if time.Now().Before(s.ltcPriceCacheExpiry) && s.ltcPriceCache > 0 {
		cachedPrice := s.ltcPriceCache
		s.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ltc_usd": cachedPrice,
		})
		return
	}
	lastPrice := s.ltcPriceCache
	s.mu.RUnlock()

	// Fetch LTC price from CoinGecko API
	resp, err := http.Get("https://api.coingecko.com/api/v3/simple/price?ids=litecoin&vs_currencies=usd")
	if err != nil {
		if lastPrice > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ltc_usd": lastPrice})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to fetch price"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if lastPrice > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ltc_usd": lastPrice})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprintf("API returned status %d", resp.StatusCode)})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if lastPrice > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ltc_usd": lastPrice})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to read response"})
		return
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		if lastPrice > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ltc_usd": lastPrice})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to parse response"})
		return
	}

	litecoinData, ok := data["litecoin"]
	if !ok {
		if lastPrice > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ltc_usd": lastPrice})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Litecoin data not found"})
		return
	}

	litecoinMap, ok := litecoinData.(map[string]interface{})
	if !ok {
		if lastPrice > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ltc_usd": lastPrice})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid response format"})
		return
	}

	usdPrice, ok := litecoinMap["usd"]
	if !ok {
		if lastPrice > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ltc_usd": lastPrice})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "USD price not found"})
		return
	}

	ltcPrice, ok := usdPrice.(float64)
	if !ok {
		if lastPrice > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"ltc_usd": lastPrice})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid price format"})
		return
	}

	s.mu.Lock()
	s.ltcPriceCache = ltcPrice
	s.ltcPriceCacheExpiry = time.Now().Add(5 * time.Minute)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ltc_usd": ltcPrice})
}

// handleGetUser gets user information including addresses
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var ltcAddr, kcnAddr sql.NullString
	var ltcReceiveAddr, kcnReceiveAddr sql.NullString
	err := s.db.QueryRow(`SELECT litecoin_address, kernelcoin_address, litecoin_receive_address, kernelcoin_receive_address FROM users WHERE id = ?`, session.UserID).
		Scan(&ltcAddr, &kcnAddr, &ltcReceiveAddr, &kcnReceiveAddr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":                     session.UserID,
		"username":                    session.Username,
		"litecoin_address":            ltcAddr.String,
		"kernelcoin_address":          kcnAddr.String,
		"litecoin_receive_address":    ltcReceiveAddr.String,
		"kernelcoin_receive_address":  kcnReceiveAddr.String,
	})
}

// handleWithdraw handles withdrawal requests
func (s *Server) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Coin   string  `json:"coin"`
		Amount float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// Check minimum withdrawal amount
	if req.Amount < 0.001 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "We have to pay transaction fees to send coin. The smallest amount allowed to withdraw is 0.001"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	balance, err := s.getUserBalance(session.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Balance not found"})
		return
	}

	// Calculate reserved amounts
	var reserved float64
	rows, err := s.db.Query(`SELECT coin_selling, amount_selling FROM trades WHERE seller_id = ? AND status = 'open'`, session.UserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var coin string
			var amount float64
			if rows.Scan(&coin, &amount) == nil && coin == req.Coin {
				reserved += amount
			}
		}
	}

	var userBalance float64
	if req.Coin == "litecoin" {
		userBalance = balance.Litecoin
	} else {
		userBalance = balance.Kernelcoin
	}

	availableBalance := userBalance - reserved
	if availableBalance < req.Amount {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Insufficient available balance"})
		return
	}

	// Check if user has enough total balance (including reserved)
	if userBalance < req.Amount {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Insufficient balance"})
		return
	}

	// Get withdrawal address
	var withdrawalAddr sql.NullString
	addressColumn := req.Coin + "_address"
	err = s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM users WHERE id = ?`, addressColumn), session.UserID).Scan(&withdrawalAddr)
	if err != nil || !withdrawalAddr.Valid || withdrawalAddr.String == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "No withdrawal address set"})
		return
	}

	address := withdrawalAddr.String

	// Handle withdrawals via RPC/Electrum for supported coins
	if !s.noWallets && (req.Coin == "kernelcoin" || req.Coin == "litecoin") {
		var txid string
		if req.Coin == "kernelcoin" {
			// Send via RPC
			txid, err = s.kernelcoinRPCClient.SendToAddress(address, req.Amount)
			if err != nil {
				log.Printf("[API] Failed to send %s via RPC: %v", req.Coin, err)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to send transaction"})
				return
			}
		} else {
			// Send via Electrum
			txid, err = s.electrumClient.PayTo(address, req.Amount)
			if err != nil {
				log.Printf("[API] Failed to send %s via Electrum: %v", req.Coin, err)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to send transaction"})
				return
			}
		}

		// Subtract from balance
		_, err = s.db.Exec(fmt.Sprintf(`UPDATE balances SET %s = %s - ? WHERE user_id = ?`, req.Coin, req.Coin), req.Amount, session.UserID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update balance"})
			return
		}

		// Log transaction with txid
		_, err = s.db.Exec(`INSERT INTO transactions (user_id, coin, amount, type, status, tx_hash) VALUES (?, ?, ?, 'withdraw', 'completed', ?)`, session.UserID, req.Coin, req.Amount, txid)
		if err != nil {
			log.Printf("Failed to log withdrawal transaction: %v", err)
		}

		log.Printf("[WITHDRAW] User: %s (ID:%d) | Coin: %s | Amount: %.8f | Address: %s | TxHash: %s | Status: SENT", session.Username, session.UserID, strings.ToUpper(req.Coin), req.Amount, address, txid)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Withdrawal sent",
			"txid":    txid,
		})
	} else if s.noWallets {
		// Fallback behavior when --no-wallets is used
		_, err = s.db.Exec(fmt.Sprintf(`UPDATE balances SET %s = %s - ? WHERE user_id = ?`, req.Coin, req.Coin), req.Amount, session.UserID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update balance"})
			return
		}

		_, err = s.db.Exec(`INSERT INTO transactions (user_id, coin, amount, type, status) VALUES (?, ?, ?, 'withdraw', 'completed')`, session.UserID, req.Coin, req.Amount)
		if err != nil {
			log.Printf("Failed to log withdrawal transaction: %v", err)
		}

		log.Printf("[WITHDRAW] User: %s (ID:%d) | Coin: %s | Amount: %.8f | Address: %s | Status: FALLBACK_SENT", session.Username, session.UserID, strings.ToUpper(req.Coin), req.Amount, address)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Withdrawal request submitted",
		})
	} else {
		// Wallet integration required but not available
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Wallet integration required for " + req.Coin})
	}
}

// handleGetTransactions gets user transaction history
func (s *Server) handleGetTransactions(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, coin, amount, type, status, COALESCE(tx_hash, '') as tx_hash, created_at
		FROM transactions
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 50
	`, session.UserID)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch transactions"})
		return
	}
	defer rows.Close()

	var transactions []map[string]interface{}
	for rows.Next() {
		var id int
		var coin, txType, status, txHash, createdAt string
		var amount float64

		err := rows.Scan(&id, &coin, &amount, &txType, &status, &txHash, &createdAt)
		if err != nil {
			continue
		}

		tx := map[string]interface{}{
			"id":         id,
			"coin":       coin,
			"amount":     amount,
			"type":       txType,
			"status":     status,
			"tx_hash":    txHash,
			"created_at": createdAt,
		}
		transactions = append(transactions, tx)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"transactions": transactions})
}

// handleGetAdminStats gets admin statistics
func (s *Server) handleGetAdminStats(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil || session.Username != "admin" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var completedTrades int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM trades WHERE status = 'completed'`).Scan(&completedTrades)
	if err != nil {
		completedTrades = 0
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"completed_trades": completedTrades,
	})
}

// handleChangePassword handles password change requests
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	if len(req.NewPassword) > 64 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Password must be less than 64 characters"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current user
	user, err := s.getUserByUsername(session.Username)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	// Verify current password
	if !verifyPassword(req.CurrentPassword, user.PasswordHash) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Current password is incorrect"})
		return
	}

	// Hash new password
	newPasswordHash, err := hashPassword(req.NewPassword)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Password hashing failed"})
		return
	}

	// Update password
	_, err = s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, newPasswordHash, session.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update password"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// handleUpdateAddresses handles wallet address update requests
func (s *Server) handleUpdateAddresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		LitecoinAddress   string `json:"litecoin_address"`
		KernelcoinAddress string `json:"kernelcoin_address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Handle individual address updates
	if req.LitecoinAddress != "" {
		if !isValidAlphanumeric(req.LitecoinAddress) || len(req.LitecoinAddress) > 64 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Litecoin address must be alphanumeric and less than 64 characters"})
			return
		}
		// Check if address is already in use
		var existingUserID int
		err := s.db.QueryRow(`SELECT id FROM users WHERE litecoin_address = ? AND id != ?`, req.LitecoinAddress, session.UserID).Scan(&existingUserID)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Litecoin address is already in use by another user"})
			return
		}
		_, err = s.db.Exec(`UPDATE users SET litecoin_address = ? WHERE id = ?`, req.LitecoinAddress, session.UserID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update litecoin address"})
			return
		}
		log.Printf("[ADDRESS] User: %s (ID:%d) | Updated LTC withdrawal address: %s", session.Username, session.UserID, req.LitecoinAddress)
	}

	if req.KernelcoinAddress != "" {
		if !isValidAlphanumeric(req.KernelcoinAddress) || len(req.KernelcoinAddress) > 64 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Kernelcoin address must be alphanumeric and less than 64 characters"})
			return
		}
		// Check if address is already in use
		var existingUserID int
		err := s.db.QueryRow(`SELECT id FROM users WHERE kernelcoin_address = ? AND id != ?`, req.KernelcoinAddress, session.UserID).Scan(&existingUserID)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Kernelcoin address is already in use by another user"})
			return
		}
		_, err = s.db.Exec(`UPDATE users SET kernelcoin_address = ? WHERE id = ?`, req.KernelcoinAddress, session.UserID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update kernelcoin address"})
			return
		}
		log.Printf("[ADDRESS] User: %s (ID:%d) | Updated KCN withdrawal address: %s", session.Username, session.UserID, req.KernelcoinAddress)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// handleGenerateReceiveAddress generates a receive address for a coin
func (s *Server) handleGenerateReceiveAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Coin string `json:"coin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if address already exists
	var existingAddr sql.NullString
	column := req.Coin + "_receive_address"
	err := s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM users WHERE id = ?`, column), session.UserID).Scan(&existingAddr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to check existing address"})
		return
	}

	if existingAddr.Valid && existingAddr.String != "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Receive address already exists"})
		return
	}

	// Generate address using RPC wallet for supported coins, or random for fallback
	var address string
	if !s.noWallets {
		if req.Coin == "kernelcoin" {
			// Use RPC wallet to generate real kernelcoin address
			address, err = s.kernelcoinRPCClient.GetNewAddress("", "legacy")
			if err != nil {
				log.Printf("[API] Failed to generate kernelcoin address via RPC: %v", err)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate kernelcoin address"})
				return
			}
		} else if req.Coin == "litecoin" {
			// Use Electrum to generate real litecoin address
			address, err = s.electrumClient.CreateNewAddress()
			if err != nil {
				log.Printf("[API] Failed to generate litecoin address via Electrum: %v", err)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate litecoin address"})
				return
			}
		} else {
			// Unsupported coin
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Unsupported coin: " + req.Coin})
			return
		}
	} else {
		// Generate random address (fallback when --no-wallets is used)
		address, err = generateSessionToken()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate address"})
			return
		}
	}

	// Store in database
	_, err = s.db.Exec(fmt.Sprintf(`UPDATE users SET %s = ? WHERE id = ?`, column), address, session.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save address"})
		return
	}

	log.Printf("[DEPOSIT] User: %s (ID:%d) | Coin: %s | Generated receive address: %s", session.Username, session.UserID, strings.ToUpper(req.Coin), address)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// handleCheckConfirmations checks for confirmations and adds balance
func (s *Server) handleCheckConfirmations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := s.getSession(r)
	if session == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Coin string `json:"coin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if receive address exists
	var receiveAddr sql.NullString
	column := req.Coin + "_receive_address"
	err := s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM users WHERE id = ?`, column), session.UserID).Scan(&receiveAddr)
	if err != nil || !receiveAddr.Valid || receiveAddr.String == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "No receive address found"})
		return
	}

	address := receiveAddr.String

	// Check for transactions using RPC for supported coins (unless --no-wallets is used)
	if !s.noWallets && (req.Coin == "kernelcoin" || req.Coin == "litecoin") {
		var rpcClient *CoinRPCClient
		var coinSymbol string
		var confirmedAmount, unconfirmedAmount, pendingAmount float64
		var txHash string

		if req.Coin == "kernelcoin" {
			rpcClient = s.kernelcoinRPCClient
			coinSymbol = "KCN"
			// Check confirmed transactions (2+ confirmations)
			confirmedAmount, err = rpcClient.GetReceivedByAddress(address, 2)
			if err != nil {
				log.Printf("[API] Failed to check confirmed transactions: %v", err)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to check transactions"})
				return
			}
			// Check unconfirmed transactions (0+ confirmations)
			unconfirmedAmount, err = rpcClient.GetReceivedByAddress(address, 0)
			if err != nil {
				log.Printf("[API] Failed to check unconfirmed transactions: %v", err)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to check transactions"})
				return
			}
			pendingAmount = unconfirmedAmount - confirmedAmount
		} else {
			coinSymbol = "LTC"
			// Use Electrum for Litecoin
			confirmedAmount, unconfirmedAmount, err = s.electrumClient.GetAddressBalance(address)
			if err != nil {
				log.Printf("[API] Failed to check Litecoin balance: %v", err)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to check transactions"})
				return
			}
			pendingAmount = unconfirmedAmount
		}

		if confirmedAmount > 0 {
			// Get transaction details to find txid
			if req.Coin == "kernelcoin" {
				txList, err := rpcClient.ListReceivedByAddress(2, false, false)
				if err == nil {
					for _, tx := range txList {
						if txMap, ok := tx.(map[string]interface{}); ok {
							if addr, ok := txMap["address"].(string); ok && addr == address {
								if txids, ok := txMap["txids"].([]interface{}); ok && len(txids) > 0 {
									if txid, ok := txids[0].(string); ok {
										txHash = txid
										break
									}
								}
							}
						}
					}
				}
			} else {
				// Get transaction hash from Electrum history
				history, err := s.electrumClient.GetAddressHistory(address)
				if err == nil && len(history) > 0 {
					if txHashVal, ok := history[0]["tx_hash"]; ok {
						if hash, ok := txHashVal.(string); ok {
							txHash = hash
						}
					}
				}
			}

			// Add confirmed amount to balance
			_, err = s.db.Exec(fmt.Sprintf(`UPDATE balances SET %s = %s + ? WHERE user_id = ?`, req.Coin, req.Coin), confirmedAmount, session.UserID)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update balance"})
				return
			}

			// Log transaction with hash
			_, err = s.db.Exec(`INSERT INTO transactions (user_id, coin, amount, type, status, tx_hash) VALUES (?, ?, ?, 'deposit', 'confirmed', ?)`, session.UserID, req.Coin, confirmedAmount, txHash)
			if err != nil {
				log.Printf("Failed to log deposit transaction: %v", err)
			}

			log.Printf("[DEPOSIT] User: %s (ID:%d) | Coin: %s | Amount: %.8f | Address: %s | TxHash: %s | Status: CONFIRMED", session.Username, session.UserID, strings.ToUpper(req.Coin), confirmedAmount, address, txHash)

			// Clear receive address
			_, err = s.db.Exec(fmt.Sprintf(`UPDATE users SET %s = NULL WHERE id = ?`, column), session.UserID)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to clear receive address"})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("Received %.8f %s (confirmed)", confirmedAmount, coinSymbol),
				"amount":  confirmedAmount,
				"status":  "confirmed",
			})
			return
		} else if pendingAmount > 0 {
			log.Printf("[DEPOSIT] User: %s (ID:%d) | Coin: %s | Amount: %.8f | Address: %s | Status: PENDING", session.Username, session.UserID, strings.ToUpper(req.Coin), pendingAmount, address)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("%.8f %s pending confirmation", pendingAmount, coinSymbol),
				"amount":  pendingAmount,
				"status":  "pending",
			})
			return
		} else {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "No transactions found for this address"})
			return
		}
	} else if s.noWallets {
		// Fallback behavior when --no-wallets is used (add 50 coins)
		_, err = s.db.Exec(fmt.Sprintf(`UPDATE balances SET %s = %s + 50 WHERE user_id = ?`, req.Coin, req.Coin), session.UserID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update balance"})
			return
		}

		_, err = s.db.Exec(`INSERT INTO transactions (user_id, coin, amount, type, status) VALUES (?, ?, 50, 'deposit', 'confirmed')`, session.UserID, req.Coin)
		if err != nil {
			log.Printf("Failed to log deposit transaction: %v", err)
		}

		log.Printf("[DEPOSIT] User: %s (ID:%d) | Coin: %s | Amount: 50.00000000 | Address: %s | Status: FALLBACK_CONFIRMED", session.Username, session.UserID, strings.ToUpper(req.Coin), address)

		_, err = s.db.Exec(fmt.Sprintf(`UPDATE users SET %s = NULL WHERE id = ?`, column), session.UserID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to clear receive address"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	} else {
		// Wallet integration required but not available
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Wallet integration required for " + req.Coin})
	}
}

// serveHTML serves static files and HTML from disk
func (s *Server) serveHTML(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "index.html"
	} else {
		path = path[1:] // Remove leading slash
	}

	// Security: prevent directory traversal
	if strings.Contains(path, "..") {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Serve static files from disk
	if strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") {
		http.ServeFile(w, r, path)
		return
	}

	// Serve HTML files (including tab files)
	if strings.HasSuffix(path, ".html") || path == "index.html" {
		http.ServeFile(w, r, path)
		return
	}

	// Default to index.html for any other request
	http.ServeFile(w, r, "index.html")
}

// handleGetWithdrawFee returns the current withdrawal fee in LTC and USD
func (s *Server) handleGetWithdrawFee(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ltcPrice := s.ltcPriceCache
	s.mu.RUnlock()
	
	feeUSD := s.ltcWithdrawFee * ltcPrice
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ltc_withdraw_fee": s.ltcWithdrawFee,
		"ltc_withdraw_fee_usd": feeUSD,
	})
}
