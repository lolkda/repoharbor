package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"repoharbor/internal/auth"
	"repoharbor/internal/store"
	db "repoharbor/internal/store/db"
)

type setupStatusResponse struct {
	NeedsSetup bool `json:"needs_setup"`
}

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.Q.CountAdmins(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "读取管理员状态失败")
		return
	}
	writeJSON(w, http.StatusOK, setupStatusResponse{NeedsSetup: count == 0})
}

type setupRequest struct {
	Password string `json:"password"`
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.Q.CountAdmins(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "读取管理员状态失败")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "ALREADY_SETUP", "管理员已存在")
		return
	}
	var req setupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "请求格式不正确")
		return
	}
	if len(req.Password) < 12 {
		writeError(w, http.StatusBadRequest, "WEAK_PASSWORD", "密码至少需要 12 个字符")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "HASH_FAILED", "密码处理失败")
		return
	}
	now := store.Now()
	admin, err := s.store.Q.CreateAdmin(r.Context(), db.CreateAdminParams{
		Username: "admin", PasswordHash: hash, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "创建管理员失败")
		return
	}
	csrf, err := s.createSession(w, r, admin.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SESSION_ERROR", "创建会话失败")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"username": admin.Username, "csrf_token": csrf})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	key := clientKey(r)
	if ok, wait := s.limiter.Allow(key); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "登录失败次数过多，请稍后再试")
		return
	}
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "请求格式不正确")
		return
	}
	if req.Username == "" {
		req.Username = "admin"
	}
	admin, err := s.store.Q.GetAdminByUsername(r.Context(), req.Username)
	if err != nil || !auth.VerifyPassword(admin.PasswordHash, req.Password) {
		s.limiter.Fail(key)
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}
	s.limiter.Reset(key)
	csrf, err := s.createSession(w, r, admin.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SESSION_ERROR", "创建会话失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"username": admin.Username, "csrf_token": csrf})
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request, adminID int64) (string, error) {
	raw, hash, err := auth.NewSessionToken()
	if err != nil {
		return "", err
	}
	csrf, err := auth.NewCSRFToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	if err := s.store.Q.CreateSession(r.Context(), db.CreateSessionParams{
		ID: hash, AdminID: adminID, CsrfToken: csrf,
		CreatedAt: now.UTC().Format(time.RFC3339),
		ExpiresAt: now.Add(s.cfg.SessionTTL).UTC().Format(time.RFC3339),
	}); err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name: auth.SessionCookie, Value: raw, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
		MaxAge: int(s.cfg.SessionTTL.Seconds()), Expires: now.Add(s.cfg.SessionTTL),
	})
	return csrf, nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	admin := currentAdmin(r)
	session := currentSession(r)
	writeJSON(w, http.StatusOK, map[string]any{"username": admin.Username, "csrf_token": session.CsrfToken})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	_ = s.store.Q.DeleteSession(r.Context(), session.ID)
	http.SetCookie(w, &http.Cookie{
		Name: auth.SessionCookie, Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "请求格式不正确")
		return
	}
	if len(req.NewPassword) < 12 {
		writeError(w, http.StatusBadRequest, "WEAK_PASSWORD", "新密码至少需要 12 个字符")
		return
	}
	admin := currentAdmin(r)
	if !auth.VerifyPassword(admin.PasswordHash, req.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "INVALID_PASSWORD", "当前密码不正确")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "HASH_FAILED", "密码处理失败")
		return
	}
	if err := s.store.Q.UpdateAdminPassword(r.Context(), db.UpdateAdminPasswordParams{
		PasswordHash: hash, UpdatedAt: store.Now(), ID: admin.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "修改密码失败")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
