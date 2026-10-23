package api

import (
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"repoharbor/internal/gitrepo"
	db "repoharbor/internal/store/db"
)

func (s *Server) repoByID(w http.ResponseWriter, r *http.Request) (db.Repository, bool) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "仓库 ID 不合法")
		return db.Repository{}, false
	}
	repo, err := s.store.Q.GetRepository(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "仓库不存在")
		return db.Repository{}, false
	}
	return repo, true
}

func (s *Server) selectedRef(r *http.Request, repo db.Repository) string {
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref != "" {
		return ref
	}
	if repo.DefaultBranch != "" {
		return repo.DefaultBranch
	}
	return "HEAD"
}

func (s *Server) refs(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoByID(w, r)
	if !ok {
		return
	}
	refs, defaultBranch, err := s.git.Refs(r.Context(), repo.Owner, repo.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GIT_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"refs": refs, "default_branch": defaultBranch})
}

func (s *Server) tree(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoByID(w, r)
	if !ok {
		return
	}
	entries, err := s.git.Tree(r.Context(), repo.Owner, repo.Name, s.selectedRef(r, repo), r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "GIT_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoByID(w, r)
	if !ok {
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "INVALID_PATH", "文件路径不能为空")
		return
	}
	result, err := s.git.File(r.Context(), repo.Owner, repo.Name, s.selectedRef(r, repo), filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "FILE_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoByID(w, r)
	if !ok {
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "INVALID_PATH", "文件路径不能为空")
		return
	}
	data, err := s.git.RawFile(r.Context(), repo.Owner, repo.Name, s.selectedRef(r, repo), filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "FILE_NOT_FOUND", err.Error())
		return
	}
	name := path.Base(filePath)
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+strings.ReplaceAll(name, "\"", "")+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func (s *Server) readme(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoByID(w, r)
	if !ok {
		return
	}
	result, err := s.git.Readme(r.Context(), repo.Owner, repo.Name, s.selectedRef(r, repo))
	if err != nil {
		writeError(w, http.StatusNotFound, "README_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) commits(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoByID(w, r)
	if !ok {
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, hasMore, err := s.git.Commits(r.Context(), repo.Owner, repo.Name, s.selectedRef(r, repo), offset, limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, "GIT_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "has_more": hasMore})
}

func (s *Server) commit(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoByID(w, r)
	if !ok {
		return
	}
	detail, err := s.git.CommitDetail(r.Context(), repo.Owner, repo.Name, chi.URLParam(r, "sha"))
	if err != nil {
		writeError(w, http.StatusNotFound, "COMMIT_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) commitDiff(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoByID(w, r)
	if !ok {
		return
	}
	detail, err := s.git.CommitDetail(r.Context(), repo.Owner, repo.Name, chi.URLParam(r, "sha"))
	if err != nil {
		writeError(w, http.StatusNotFound, "COMMIT_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diff": detail.Diff, "diff_truncated": detail.DiffTruncated})
}

var _ = gitrepo.SyncError{}
