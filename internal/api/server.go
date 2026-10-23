package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"repoharbor/internal/auth"
	"repoharbor/internal/config"
	"repoharbor/internal/events"
	"repoharbor/internal/gitrepo"
	"repoharbor/internal/scheduler"
	"repoharbor/internal/store"
	db "repoharbor/internal/store/db"
)

type ctxKey int

const (
	ctxAdmin ctxKey = iota
	ctxSession
)

// Server wires HTTP handlers to the store, Git service and scheduler.
type Server struct {
	cfg     config.Config
	store   *store.Store
	git     *gitrepo.Service
	sched   *scheduler.Scheduler
	broker  *events.Broker
	limiter *auth.Limiter
	webFS   fs.FS
}

func New(cfg config.Config, st *store.Store, git *gitrepo.Service, sched *scheduler.Scheduler, broker *events.Broker, webFS fs.FS) *Server {
	return &Server{cfg: cfg, store: st, git: git, sched: sched, broker: broker, limiter: auth.NewLimiter(), webFS: webFS}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Logger)

	r.Get("/api/v1/health", s.health)
	r.Get("/api/v1/openapi.json", s.openapi)
	r.Get("/api/v1/setup/status", s.setupStatus)
	r.Post("/api/v1/setup", s.setup)
	r.Post("/api/v1/auth/login", s.login)

	r.Group(func(pr chi.Router) {
		pr.Use(s.requireAuth)
		pr.Get("/api/v1/auth/me", s.me)
		pr.Post("/api/v1/auth/logout", s.logout)
		pr.Patch("/api/v1/settings/password", s.changePassword)
		pr.Get("/api/v1/settings", s.settings)
		pr.Get("/api/v1/events", s.events)

		pr.Get("/api/v1/repositories", s.listRepositories)
		pr.Post("/api/v1/repositories", s.createRepository)
		pr.Get("/api/v1/repositories/{id}", s.getRepository)
		pr.Patch("/api/v1/repositories/{id}", s.updateRepository)
		pr.Delete("/api/v1/repositories/{id}", s.deleteRepository)
		pr.Post("/api/v1/repositories/{id}/sync", s.syncRepository)
		pr.Post("/api/v1/repositories/{id}/pause", s.pauseRepository)
		pr.Post("/api/v1/repositories/{id}/resume", s.resumeRepository)
		pr.Post("/api/v1/repositories/{id}/force-reset", s.forceResetRepository)
		pr.Get("/api/v1/repositories/{id}/sync-runs", s.listSyncRuns)
		pr.Get("/api/v1/repositories/{id}/refs", s.refs)
		pr.Get("/api/v1/repositories/{id}/tree", s.tree)
		pr.Get("/api/v1/repositories/{id}/file", s.file)
		pr.Get("/api/v1/repositories/{id}/download", s.download)
		pr.Get("/api/v1/repositories/{id}/readme", s.readme)
		pr.Get("/api/v1/repositories/{id}/commits", s.commits)
		pr.Get("/api/v1/repositories/{id}/commits/{sha}", s.commit)
		pr.Get("/api/v1/repositories/{id}/commits/{sha}/diff", s.commitDiff)
	})

	r.NotFound(s.serveSPA)
	return r
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": store.Now()})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookie)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
			return
		}
		session, err := s.store.Q.GetSession(r.Context(), db.GetSessionParams{
			ID: auth.HashToken(cookie.Value), ExpiresAt: store.Now(),
		})
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "会话已过期，请重新登录")
			return
		}
		admin, err := s.store.Q.GetAdminByID(r.Context(), session.AdminID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "管理员不存在")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			token := r.Header.Get("X-CSRF-Token")
			if subtle.ConstantTimeCompare([]byte(token), []byte(session.CsrfToken)) != 1 {
				writeError(w, http.StatusForbidden, "CSRF_INVALID", "CSRF Token 无效")
				return
			}
		}
		ctx := context.WithValue(r.Context(), ctxAdmin, admin)
		ctx = context.WithValue(ctx, ctxSession, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func currentAdmin(r *http.Request) db.AdminUser {
	v, _ := r.Context().Value(ctxAdmin).(db.AdminUser)
	return v
}

func currentSession(r *http.Request) db.Session {
	v, _ := r.Context().Value(ctxSession).(db.Session)
	return v
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{Code: code, Message: message})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil && id > 0
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "接口不存在")
		return
	}
	if s.webFS == nil {
		http.Error(w, "frontend not built", http.StatusNotFound)
		return
	}
	sub, err := fs.Sub(s.webFS, "web/dist")
	if err != nil {
		http.Error(w, "frontend not built", http.StatusNotFound)
		return
	}
	clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if clean != "" && clean != "." {
		if f, err := sub.Open(clean); err == nil {
			_ = f.Close()
			http.FileServer(http.FS(sub)).ServeHTTP(w, r)
			return
		}
	}
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		http.Error(w, "frontend not built", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// used by handlers that need a fallback logger
func logError(msg string, err error) {
	if err != nil {
		slog.Error(msg, "error", err)
	}
}

var errNotFound = errors.New("not found")
