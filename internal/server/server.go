package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"SpotiLayer/internal/config"
	"SpotiLayer/internal/spotify"
)

type Server struct {
	mu            sync.RWMutex
	httpServer    *http.Server
	listener      net.Listener
	spotifyClient *spotify.Client
	config        *config.Config
	running       bool
	templates     *template.Template
	fs            fs.FS
	currentPort   int
	currentHost   string

	onStatusChange func(running bool, addr string)
}

func NewServer(cfg *config.Config, spClient *spotify.Client, embeddedFS fs.FS) (*Server, error) {
	s := &Server{
		config:        cfg,
		spotifyClient: spClient,
		fs:            embeddedFS,
	}

	if err := s.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return s, nil
}

func (s *Server) SetOnStatusChange(fn func(running bool, addr string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onStatusChange = fn
}

func (s *Server) loadTemplates() error {
	tmpl := template.New("")
	// Add helper functions if needed
	tmpl.Funcs(template.FuncMap{
		"url_for": func(endpoint, filename string) string {
			return "/static/" + filename
		},
	})

	var err error
	tmpl, err = tmpl.ParseFS(s.fs, "templates/*.html")
	if err != nil {
		return err
	}
	s.templates = tmpl
	return nil
}

func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Server) GetAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.running {
		return ""
	}
	host := s.currentHost
	if host == "" || host == "0.0.0.0" || host == "localhost" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d", host, s.currentPort)
}

func (s *Server) Start(host string, port int) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("server is already running")
	}

	if host == "" || host == "localhost" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = 8888
	}

	s.currentHost = host
	s.currentPort = port

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	httpSrv := &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	s.httpServer = httpSrv
	s.listener = listener
	s.running = true
	notifyFn := s.onStatusChange
	serverAddr := fmt.Sprintf("http://%s:%d", host, port)
	s.mu.Unlock()

	if notifyFn != nil {
		notifyFn(true, serverAddr)
	}

	go func() {
		if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[Server] error: %v", err)
		}
		s.mu.Lock()
		s.running = false
		notify := s.onStatusChange
		s.mu.Unlock()
		if notify != nil {
			notify(false, "")
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running || s.httpServer == nil {
		s.mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(ctx)
	s.running = false
	s.httpServer = nil
	s.listener = nil
	notify := s.onStatusChange
	s.mu.Unlock()

	if notify != nil {
		notify(false, "")
	}

	return err
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Serve static files
	staticSub, err := fs.Sub(s.fs, "static")
	if err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	}

	// Application routes
	mux.HandleFunc("/", s.handleMain)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/callback", s.handleCallback)
	mux.HandleFunc("/logout", s.handleLogout)

	// API routes
	mux.HandleFunc("/api/current", s.handleAPICurrent)
	mux.HandleFunc("/api/status", s.handleAPIStatus)
}

func (s *Server) handleMain(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	token := ""
	if cookie, err := r.Cookie("token"); err == nil {
		token = cookie.Value
	}

	// If no cookie but client has a valid token, set cookie for convenience
	clientToken, clientRefresh := s.spotifyClient.GetTokens()
	if token == "" && clientToken != "" {
		token = clientToken
		setCookie(w, "token", clientToken)
		if clientRefresh != "" {
			setCookie(w, "refresh_token", clientRefresh)
		}
	}

	if token == "" {
		s.renderTemplate(w, "login.html", nil)
		return
	}

	// Fetch currently playing track
	track, err := s.spotifyClient.GetCurrentlyPlaying(token)
	if errors.Is(err, spotify.ErrTokenExpired) {
		// Attempt refresh
		refreshToken := ""
		if c, err := r.Cookie("refresh_token"); err == nil {
			refreshToken = c.Value
		}
		if refreshToken == "" {
			refreshToken = clientRefresh
		}

		if refreshToken != "" {
			tokenResp, refreshErr := s.spotifyClient.RefreshAccessToken(refreshToken)
			if refreshErr == nil {
				setCookie(w, "token", tokenResp.AccessToken)
				if tokenResp.RefreshToken != "" {
					setCookie(w, "refresh_token", tokenResp.RefreshToken)
				}
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}

		s.renderTemplate(w, "error.html", nil)
		return
	}

	if err != nil {
		s.renderTemplate(w, "error.html", nil)
		return
	}

	s.renderTemplate(w, "index.html", track)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := spotify.GenerateRandomState(16)
	s.mu.RLock()
	host := s.currentHost
	port := s.currentPort
	s.mu.RUnlock()

	if host == "" || host == "0.0.0.0" || host == "localhost" {
		host = "127.0.0.1"
	}
	redirectURI := fmt.Sprintf("http://%s:%d/callback", host, port)
	authURL := s.spotifyClient.BuildAuthorizeURL(redirectURI, state)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if state == "" || code == "" {
		http.Redirect(w, r, "/#error=state_mismatch", http.StatusFound)
		return
	}

	s.mu.RLock()
	host := s.currentHost
	port := s.currentPort
	s.mu.RUnlock()

	if host == "" || host == "0.0.0.0" || host == "localhost" {
		host = "127.0.0.1"
	}
	redirectURI := fmt.Sprintf("http://%s:%d/callback", host, port)

	tokenResp, err := s.spotifyClient.ExchangeCode(code, redirectURI)
	if err != nil {
		log.Printf("[Server] code exchange error: %v", err)
		s.renderTemplate(w, "error.html", nil)
		return
	}

	setCookie(w, "token", tokenResp.AccessToken)
	setCookie(w, "refresh_token", tokenResp.RefreshToken)
	setCookie(w, "client_id", s.config.ClientID)

	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	expireCookie(w, "token")
	expireCookie(w, "refresh_token")
	s.spotifyClient.ClearTokens()
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleAPICurrent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	track, err := s.spotifyClient.GetCurrentlyPlaying("")
	if err != nil {
		if errors.Is(err, spotify.ErrNotPlaying) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(track)
}

func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.mu.RLock()
	defer s.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"running":      s.running,
		"address":      s.GetAddress(),
		"port":         s.currentPort,
		"is_logged_in": s.spotifyClient.IsAuthorized(),
	})
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("[Server] template render error (%s): %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func setCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: false, // Must be accessible by JavaScript Cookies.get('token')
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600, // 30 days
	})
}

func expireCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}
