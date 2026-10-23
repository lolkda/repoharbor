package api

import (
	"net/http"
	"syscall"
)

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	repos, _ := s.store.Q.ListRepositories(r.Context())
	var stat syscall.Statfs_t
	total, free, usedPercent := uint64(0), uint64(0), float64(0)
	if err := syscall.Statfs(s.cfg.DataDir, &stat); err == nil {
		total = stat.Blocks * uint64(stat.Bsize)
		free = stat.Bavail * uint64(stat.Bsize)
		if total > 0 {
			usedPercent = float64(total-free) / float64(total) * 100
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data_dir":          s.cfg.DataDir,
		"disk_total":        total,
		"disk_free":         free,
		"disk_used_percent": usedPercent,
		"repo_count":        len(repos),
	})
}
