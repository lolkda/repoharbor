package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"repoharbor/internal/gitrepo"
	"repoharbor/internal/schedule"
	"repoharbor/internal/scheduler"
	"repoharbor/internal/store"
	db "repoharbor/internal/store/db"
)

type repoResponse struct {
	ID               int64  `json:"id"`
	URL              string `json:"url"`
	Owner            string `json:"owner"`
	Name             string `json:"name"`
	DisplayName      string `json:"display_name"`
	Notes            string `json:"notes"`
	CronExpression   string `json:"cron_expression"`
	Enabled          bool   `json:"enabled"`
	Status           string `json:"status"`
	DefaultBranch    string `json:"default_branch"`
	LastSyncAt       string `json:"last_sync_at"`
	NextSyncAt       string `json:"next_sync_at"`
	LastErrorCode    string `json:"last_error_code"`
	LastErrorMessage string `json:"last_error_message"`
	SizeBytes        int64  `json:"size_bytes"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

func toRepoResponse(repo db.Repository) repoResponse {
	return repoResponse{
		ID: repo.ID, URL: repo.Url, Owner: repo.Owner, Name: repo.Name,
		DisplayName: repo.DisplayName, Notes: repo.Notes, CronExpression: repo.CronExpression,
		Enabled: repo.Enabled == 1, Status: repo.Status, DefaultBranch: repo.DefaultBranch,
		LastSyncAt: repo.LastSyncAt, NextSyncAt: repo.NextSyncAt,
		LastErrorCode: repo.LastErrorCode, LastErrorMessage: repo.LastErrorMessage,
		SizeBytes: repo.SizeBytes, CreatedAt: repo.CreatedAt, UpdatedAt: repo.UpdatedAt,
	}
}

func (s *Server) listRepositories(w http.ResponseWriter, r *http.Request) {
	repos, err := s.store.Q.ListRepositories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "读取仓库列表失败")
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	filtered := make([]db.Repository, 0, len(repos))
	for _, repo := range repos {
		if status != "" && status != "all" && repo.Status != status {
			continue
		}
		if q != "" {
			hay := strings.ToLower(repo.DisplayName + " " + repo.Owner + " " + repo.Name + " " + repo.Url)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		filtered = append(filtered, repo)
	}
	sortKey := r.URL.Query().Get("sort")
	if sortKey == "" {
		sortKey = "name"
	}
	desc := r.URL.Query().Get("order") == "desc"
	sort.SliceStable(filtered, func(i, j int) bool {
		var less bool
		switch sortKey {
		case "last_sync":
			less = filtered[i].LastSyncAt < filtered[j].LastSyncAt
		case "created":
			less = filtered[i].CreatedAt < filtered[j].CreatedAt
		default:
			less = strings.ToLower(filtered[i].DisplayName) < strings.ToLower(filtered[j].DisplayName)
		}
		if desc {
			return !less
		}
		return less
	})
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := make([]repoResponse, 0, end-start)
	for _, repo := range filtered[start:end] {
		items = append(items, toRepoResponse(repo))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": page, "page_size": pageSize})
}

type createRepoRequest struct {
	URL            string `json:"url"`
	DisplayName    string `json:"display_name"`
	Notes          string `json:"notes"`
	CronExpression string `json:"cron_expression"`
}

func (s *Server) createRepository(w http.ResponseWriter, r *http.Request) {
	var req createRepoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "请求格式不正确")
		return
	}
	owner, name, normalized, err := gitrepo.ParseURL(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_URL", err.Error())
		return
	}
	req.CronExpression = schedule.Normalize(req.CronExpression)
	if req.CronExpression == "" {
		req.CronExpression = schedule.Default
	}
	if err := schedule.Validate(req.CronExpression); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CRON", err.Error())
		return
	}
	if len(req.Notes) > 1000 {
		writeError(w, http.StatusBadRequest, "NOTES_TOO_LONG", "备注最多 1000 个字符")
		return
	}
	if _, err := s.store.Q.GetRepositoryByURL(r.Context(), normalized); err == nil {
		writeError(w, http.StatusConflict, "DUPLICATE_REPO", "该仓库已存在")
		return
	}
	if err := s.git.ValidateRemote(r.Context(), normalized); err != nil {
		writeError(w, http.StatusBadRequest, "UNREACHABLE", err.Error())
		return
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		req.DisplayName = owner + "/" + name
	}
	now := store.Now()
	nextSync := ""
	if next, nextErr := schedule.Next(req.CronExpression, time.Now()); nextErr == nil {
		nextSync = next.UTC().Format(time.RFC3339)
	}
	repo, err := s.store.Q.CreateRepository(r.Context(), db.CreateRepositoryParams{
		Url: normalized, Owner: owner, Name: name, DisplayName: req.DisplayName, Notes: req.Notes,
		CronExpression: req.CronExpression, Enabled: 1, Status: "pending",
		DefaultBranch: "", LastSyncAt: "", NextSyncAt: nextSync, LastErrorCode: "", LastErrorMessage: "",
		SizeBytes: 0, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "创建仓库失败")
		return
	}
	_ = s.sched.Trigger(r.Context(), repo.ID, "initial")
	writeJSON(w, http.StatusCreated, toRepoResponse(repo))
}

func (s *Server) getRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "仓库 ID 不合法")
		return
	}
	repo, err := s.store.Q.GetRepository(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "仓库不存在")
		return
	}
	writeJSON(w, http.StatusOK, toRepoResponse(repo))
}

type updateRepoRequest struct {
	DisplayName    *string `json:"display_name"`
	Notes          *string `json:"notes"`
	CronExpression *string `json:"cron_expression"`
	Enabled        *bool   `json:"enabled"`
}

func (s *Server) updateRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "仓库 ID 不合法")
		return
	}
	repo, err := s.store.Q.GetRepository(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "仓库不存在")
		return
	}
	var req updateRepoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "请求格式不正确")
		return
	}
	displayName := repo.DisplayName
	notes := repo.Notes
	cronExpression := repo.CronExpression
	enabled := repo.Enabled
	recalcNext := false
	if req.DisplayName != nil {
		displayName = strings.TrimSpace(*req.DisplayName)
		if displayName == "" {
			displayName = repo.Owner + "/" + repo.Name
		}
	}
	if req.Notes != nil {
		if len(*req.Notes) > 1000 {
			writeError(w, http.StatusBadRequest, "NOTES_TOO_LONG", "备注最多 1000 个字符")
			return
		}
		notes = *req.Notes
	}
	if req.CronExpression != nil {
		candidate := schedule.Normalize(*req.CronExpression)
		if candidate == "" {
			candidate = schedule.Default
		}
		if err := schedule.Validate(candidate); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_CRON", err.Error())
			return
		}
		if candidate != cronExpression {
			cronExpression = candidate
			recalcNext = true
		}
	}
	if req.Enabled != nil {
		newEnabled := int64(0)
		if *req.Enabled {
			newEnabled = 1
		}
		if newEnabled != enabled {
			enabled = newEnabled
			recalcNext = true
		}
	}
	updated, err := s.store.Q.UpdateRepositoryConfig(r.Context(), db.UpdateRepositoryConfigParams{
		DisplayName: displayName, Notes: notes, CronExpression: cronExpression, Enabled: enabled,
		UpdatedAt: store.Now(), ID: repo.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "保存仓库配置失败")
		return
	}
	if recalcNext {
		next := ""
		if enabled == 1 {
			if nextTime, nextErr := schedule.Next(cronExpression, time.Now()); nextErr == nil {
				next = nextTime.UTC().Format(time.RFC3339)
			}
		}
		_ = s.store.Q.UpdateRepositoryNextSync(r.Context(), db.UpdateRepositoryNextSyncParams{
			NextSyncAt: next, UpdatedAt: store.Now(), ID: repo.ID,
		})
		updated.NextSyncAt = next
	}
	writeJSON(w, http.StatusOK, toRepoResponse(updated))
}

func (s *Server) deleteRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "仓库 ID 不合法")
		return
	}
	repo, err := s.store.Q.GetRepository(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "仓库不存在")
		return
	}
	if repo.Status == "syncing" {
		writeError(w, http.StatusConflict, "SYNCING", "仓库正在同步，请稍后再删除")
		return
	}
	if err := s.git.RemoveMirror(repo.Owner, repo.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_FAILED", "删除本地镜像失败")
		return
	}
	if err := s.store.Q.DeleteRepository(r.Context(), repo.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "删除仓库记录失败")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) syncRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "仓库 ID 不合法")
		return
	}
	if err := s.sched.Trigger(r.Context(), id, "manual"); err != nil {
		if errors.Is(err, scheduler.ErrBusy) {
			writeError(w, http.StatusConflict, "SYNCING", "仓库正在同步")
			return
		}
		writeError(w, http.StatusNotFound, "NOT_FOUND", "仓库不存在")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (s *Server) pauseRepository(w http.ResponseWriter, r *http.Request) {
	s.setEnabled(w, r, false)
}

func (s *Server) resumeRepository(w http.ResponseWriter, r *http.Request) {
	s.setEnabled(w, r, true)
}

func (s *Server) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "仓库 ID 不合法")
		return
	}
	repo, err := s.store.Q.GetRepository(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "仓库不存在")
		return
	}
	enabledInt := int64(0)
	next := ""
	if enabled {
		enabledInt = 1
		if nextTime, nextErr := schedule.Next(repo.CronExpression, time.Now()); nextErr == nil {
			next = nextTime.UTC().Format(time.RFC3339)
		}
	}
	updated, err := s.store.Q.UpdateRepositoryConfig(r.Context(), db.UpdateRepositoryConfigParams{
		DisplayName: repo.DisplayName, Notes: repo.Notes, CronExpression: repo.CronExpression,
		Enabled: enabledInt, UpdatedAt: store.Now(), ID: repo.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "更新仓库状态失败")
		return
	}
	_ = s.store.Q.UpdateRepositoryNextSync(r.Context(), db.UpdateRepositoryNextSyncParams{
		NextSyncAt: next, UpdatedAt: store.Now(), ID: repo.ID,
	})
	updated.NextSyncAt = next
	writeJSON(w, http.StatusOK, toRepoResponse(updated))
}

func (s *Server) forceResetRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "仓库 ID 不合法")
		return
	}
	if err := s.sched.ForceReset(r.Context(), id); err != nil {
		if errors.Is(err, scheduler.ErrBusy) {
			writeError(w, http.StatusConflict, "SYNCING", "仓库正在同步")
			return
		}
		writeError(w, http.StatusNotFound, "NOT_FOUND", "仓库不存在")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (s *Server) listSyncRuns(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "仓库 ID 不合法")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	runs, err := s.store.Q.ListSyncRuns(r.Context(), db.ListSyncRunsParams{
		RepositoryID: id, Limit: int64(limit), Offset: int64(offset),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "读取同步记录失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": runs})
}
