package main

import (
	"log"
	"sync"
	"time"

	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/slide"
)

type CaptchaService struct {
	captcha  slide.Captcha
	sessions map[string]*CaptchaSession
	mu       sync.RWMutex
}

type CaptchaSession struct {
	Data   *slide.Block
	Expiry time.Time
}

type CaptchaResponse struct {
	CaptchaID   string `json:"captcha_id"`
	MasterImage string `json:"master_image"`
	TileImage   string `json:"tile_image"`
	TileX       int    `json:"tile_x"`
	TileY       int    `json:"tile_y"`
}

func NewCaptchaService() *CaptchaService {
	builder := slide.NewBuilder(
		slide.WithGenGraphNumber(1),
	)

	// Load background images
	imgs, err := imagesv2.GetImages()
	if err != nil {
		log.Fatalf("Failed to load captcha images: %v", err)
	}

	// Load tile graphics
	graphs, err := tiles.GetTiles()
	if err != nil {
		log.Fatalf("Failed to load captcha tiles: %v", err)
	}

	var newGraphs = make([]*slide.GraphImage, 0, len(graphs))
	for i := 0; i < len(graphs); i++ {
		graph := graphs[i]
		newGraphs = append(newGraphs, &slide.GraphImage{
			OverlayImage: graph.OverlayImage,
			MaskImage:    graph.MaskImage,
			ShadowImage:  graph.ShadowImage,
		})
	}

	// Set resources
	builder.SetResources(
		slide.WithGraphImages(newGraphs),
		slide.WithBackgrounds(imgs),
	)

	return &CaptchaService{
		captcha:  builder.MakeDragDrop(),
		sessions: make(map[string]*CaptchaSession),
	}
}

func (cs *CaptchaService) GenerateCaptcha() (*CaptchaResponse, error) {
	captData, err := cs.captcha.Generate()
	if err != nil {
		return nil, err
	}

	dotData := captData.GetData()
	if dotData == nil {
		return nil, err
	}

	// Generate session ID
	sessionID, err := generateSessionToken()
	if err != nil {
		return nil, err
	}

	// Store captcha data
	cs.mu.Lock()
	cs.sessions[sessionID] = &CaptchaSession{
		Data:   dotData,
		Expiry: time.Now().Add(5 * time.Minute),
	}
	cs.mu.Unlock()

	// Get base64 images
	masterBase64, err := captData.GetMasterImage().ToBase64()
	if err != nil {
		return nil, err
	}

	tileBase64, err := captData.GetTileImage().ToBase64()
	if err != nil {
		return nil, err
	}

	log.Printf("[CAPTCHA] Generated puzzle - ID: %s", sessionID)

	return &CaptchaResponse{
		CaptchaID:   sessionID,
		MasterImage: masterBase64,
		TileImage:   tileBase64,
		TileX:       dotData.X,
		TileY:       dotData.Y,
	}, nil
}

func (cs *CaptchaService) ValidateCaptcha(captchaID string, userX, userY int) bool {
	cs.mu.RLock()
	session, exists := cs.sessions[captchaID]
	cs.mu.RUnlock()

	if !exists || session.Expiry.Before(time.Now()) {
		return false
	}

	// Clean up session
	cs.mu.Lock()
	delete(cs.sessions, captchaID)
	cs.mu.Unlock()

	// Validate with padding tolerance
	log.Printf("[CAPTCHA] Validating puzzle - ID: %s, Expected: X=%d, Y=%d | Received: X=%d, Y=%d", captchaID, session.Data.X, session.Data.Y, userX, userY)
	isValid := slide.Validate(userX, userY, session.Data.X, session.Data.Y, 10)
	log.Printf("[CAPTCHA] Validation result for ID %s: %v (tolerance: 10px)", captchaID, isValid)
	return isValid
}

func (cs *CaptchaService) CleanupExpired() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	now := time.Now()
	for id, session := range cs.sessions {
		if session.Expiry.Before(now) {
			delete(cs.sessions, id)
		}
	}
}
