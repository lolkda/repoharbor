package api

import (
	"net/http"
	"strings"
)

type openAPIOperation struct {
	method  string
	path    string
	summary string
	auth    bool
}

func (s *Server) openapi(w http.ResponseWriter, _ *http.Request) {
	ops := []openAPIOperation{
		{"get", "/api/v1/health", "健康检查", false},
		{"get", "/api/v1/setup/status", "首次设置状态", false},
		{"post", "/api/v1/setup", "创建管理员", false},
		{"post", "/api/v1/auth/login", "登录", false},
		{"post", "/api/v1/auth/logout", "登出", true},
		{"get", "/api/v1/auth/me", "当前管理员", true},
		{"get", "/api/v1/repositories", "仓库列表", true},
		{"post", "/api/v1/repositories", "新增仓库", true},
		{"get", "/api/v1/repositories/{id}", "仓库详情", true},
		{"patch", "/api/v1/repositories/{id}", "修改仓库配置", true},
		{"delete", "/api/v1/repositories/{id}", "删除本地仓库", true},
		{"post", "/api/v1/repositories/{id}/sync", "立即同步", true},
		{"post", "/api/v1/repositories/{id}/pause", "暂停调度", true},
		{"post", "/api/v1/repositories/{id}/resume", "恢复调度", true},
		{"post", "/api/v1/repositories/{id}/force-reset", "强制重新镜像", true},
		{"get", "/api/v1/repositories/{id}/sync-runs", "同步历史", true},
		{"get", "/api/v1/repositories/{id}/refs", "分支和标签", true},
		{"get", "/api/v1/repositories/{id}/tree", "目录树", true},
		{"get", "/api/v1/repositories/{id}/file", "文件内容", true},
		{"get", "/api/v1/repositories/{id}/download", "文件下载", true},
		{"get", "/api/v1/repositories/{id}/readme", "README", true},
		{"get", "/api/v1/repositories/{id}/commits", "提交历史", true},
		{"get", "/api/v1/repositories/{id}/commits/{sha}", "提交详情", true},
		{"get", "/api/v1/repositories/{id}/commits/{sha}/diff", "提交差异", true},
		{"get", "/api/v1/settings", "系统设置", true},
		{"patch", "/api/v1/settings/password", "修改密码", true},
		{"get", "/api/v1/events", "SSE 事件流", true},
	}
	paths := map[string]map[string]any{}
	for _, op := range ops {
		if paths[op.path] == nil {
			paths[op.path] = map[string]any{}
		}
		paths[op.path][strings.ToLower(op.method)] = map[string]any{
			"summary": op.summary,
			"security": func() []map[string][]string {
				if op.auth {
					return []map[string][]string{{"cookieAuth": {}}}
				}
				return nil
			}(),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": "RepoHarbor API", "version": "1.0.0"},
		"paths":   paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"cookieAuth": map[string]any{"type": "apiKey", "in": "cookie", "name": "repoharbor_session"},
			},
		},
	})
}
