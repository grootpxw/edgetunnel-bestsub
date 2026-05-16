package web

import (
	"context"
	"embed"
	"encoding/json"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pangxianwei/edgetunnel-bestsub/internal/app"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/config"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/preflight"
	"github.com/pangxianwei/edgetunnel-bestsub/internal/worker"
)

func init() {
	// 强制纠正 Windows 下可能错误的 MIME 类型
	mime.AddExtensionType(".js", "application/javascript; charset=utf-8")
	mime.AddExtensionType(".css", "text/css; charset=utf-8")
	mime.AddExtensionType(".woff2", "font/woff2")
}

//go:embed static
var staticFS embed.FS

type Server struct {
	configPath string
	cfg        config.Config
	mu         sync.Mutex
	running    bool
	last       *app.RunResult
	lastError  string
}

func New(configPath string, cfg config.Config) *Server {
	return &Server{configPath: configPath, cfg: cfg}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/results/latest", s.handleLatest)
	mux.HandleFunc("/api/results/add.txt", s.handleADD)
	mux.HandleFunc("/api/config/update", s.handleConfigUpdate)
	mux.HandleFunc("/api/preflight", s.handlePreflight)
	mux.HandleFunc("/api/probe/run", s.handleRun)
	mux.HandleFunc("/api/worker/push", s.handlePush)
	mux.HandleFunc("/", s.handleIndex)

	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"config_path": s.configPath,
		"config":      s.cfg,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, map[string]any{
		"running":    s.running,
		"last_error": s.lastError,
		"has_result": s.last != nil,
		"last_candidates": func() int {
			if s.last == nil {
				return 0
			}
			return s.last.Candidates
		}(),
		"last_success": func() int {
			if s.last == nil {
				return 0
			}
			count := 0
			for _, r := range s.last.Results {
				if r.Success {
					count++
				}
			}
			return count
		}(),
	})
}

func (s *Server) handleLatest(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		http.Error(w, "no result yet", http.StatusNotFound)
		return
	}
	writeJSON(w, s.last)
}

func (s *Server) handleADD(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		http.Error(w, "no result yet", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s.last.ADDText))
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		http.Error(w, "probe is already running", http.StatusConflict)
		return
	}
	s.running = true
	s.lastError = ""
	s.mu.Unlock()

	push := r.URL.Query().Get("push") == "1" || r.URL.Query().Get("push") == "true"
	cfg := s.cfg
	if countries := parseCountries(r.URL.Query().Get("countries")); len(countries) > 0 {
		cfg.Probe.Countries = countries
	}
	mode := r.URL.Query().Get("mode")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		result, err := app.RunOnceMode(ctx, cfg, push, mode)
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			s.lastError = err.Error()
		} else {
			s.last = &result
		}
		s.running = false
	}()

	writeJSON(w, map[string]any{"started": true})
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		http.Error(w, "probe is currently running", http.StatusConflict)
		return
	}
	if s.last == nil || s.last.ADDText == "" {
		s.mu.Unlock()
		http.Error(w, "no result to push", http.StatusBadRequest)
		return
	}
	if s.cfg.Worker.Password == "" {
		s.mu.Unlock()
		http.Error(w, "未配置 Worker 密码，请在配置文件中填写 worker.password", http.StatusBadRequest)
		return
	}
	if s.cfg.Output.DryRun {
		s.mu.Unlock()
		http.Error(w, "处于演练模式 (dry_run: true)，推送已跳过。请在配置中将其改为 false 后重启", http.StatusBadRequest)
		return
	}
	last := s.last
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Minute)
	defer cancel()

	client, err := worker.New(s.cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := client.Login(ctx); err != nil {
		http.Error(w, "login: "+err.Error(), http.StatusUnauthorized)
		return
	}
	if err := client.PushADD(ctx, last.ADDText); err != nil {
		http.Error(w, "push: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.last.Pushed = true
	s.mu.Unlock()

	writeJSON(w, map[string]any{"success": true})
}

func parseCountries(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToUpper(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func (s *Server) handlePreflight(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	writeJSON(w, preflight.Run(ctx, s.cfg))
}

func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Countries []string `json:"countries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.Probe.Countries = req.Countries
	if err := s.cfg.Save(s.configPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}
