package main

import (
	"embed"
	"log"

	"spotify-obs/internal/config"
	"spotify-obs/internal/server"
	"spotify-obs/internal/spotify"
	"spotify-obs/internal/ui"
)

//go:embed templates/* static/*
var embeddedAssets embed.FS

func main() {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[Warning] Failed to load config: %v", err)
		cfg = config.DefaultConfig()
	}

	// 2. Initialize Spotify API Client
	spClient := spotify.NewClient()
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		spClient.UpdateCredentials(cfg.ClientID, cfg.ClientSecret)
	}

	// 3. Initialize HTTP Server
	srv, err := server.NewServer(cfg, spClient, embeddedAssets)
	if err != nil {
		log.Fatalf("[Error] Failed to initialize server: %v", err)
	}

	// 4. Load Korean Font for GUI
	fontBytes, err := embeddedAssets.ReadFile("static/AppleSDGothicNeoB.ttf")
	if err != nil {
		log.Printf("[Warning] Failed to load embedded font: %v", err)
	}

	// 5. Initialize and launch GUI
	appUI := ui.NewUI(cfg, spClient, srv, fontBytes)
	appUI.Run()
}
