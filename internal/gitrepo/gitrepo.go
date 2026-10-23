package gitrepo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const maxInlineFile = 2 * 1024 * 1024

var (
	segmentPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	refPattern     = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

// SyncError is a classified Git error.
type SyncError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *SyncError) Error() string { return e.Message }

func syncErr(code, message string, retryable bool) error {
	return &SyncError{Code: code, Message: message, Retryable: retryable}
}

// Service performs Git operations against local bare mirrors.
type Service struct {
	BaseDir string
}

func New(baseDir string) *Service { return &Service{BaseDir: baseDir} }

func (s *Service) mirrorPath(owner, name string) string {
	return filepath.Join(s.BaseDir, owner+"__"+name+".git")
}

// ParseURL validates and normalizes a public GitHub repository URL.
func ParseURL(raw string) (owner, name, normalized string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", syncErr("INVALID_URL", "仓库地址不能为空", false)
	}
	if !strings.HasPrefix(raw, "https://") {
		return "", "", "", syncErr("INVALID_URL", "仅支持 https://github.com/owner/repo", false)
	}
	trimmed := strings.TrimPrefix(raw, "https://")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) != 3 || !strings.EqualFold(parts[0], "github.com") {
		return "", "", "", syncErr("INVALID_URL", "仅支持 github.com 公开仓库", false)
	}
	owner = parts[1]
	name = strings.TrimSuffix(parts[2], "/")
	name = strings.TrimSuffix(name, ".git")
	if !segmentPattern.MatchString(owner) || !segmentPattern.MatchString(name) || owner == "." || name == "." {
		return "", "", "", syncErr("INVALID_URL", "仓库地址格式不正确", false)
	}
	return owner, name, "https://github.com/" + owner + "/" + name, nil
}

type logKey struct{}

// WithLog attaches a writer that receives Git command output for sync logs.
func WithLog(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, logKey{}, w)
}

func logf(ctx context.Context, format string, args ...any) {
	if w, ok := ctx.Value(logKey{}).(io.Writer); ok {
		text := strings.ReplaceAll(fmt.Sprintf(format, args...), "\x00", " ")
		_, _ = fmt.Fprintln(w, text)
	}
}

func run(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/false", "LC_ALL=C")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	logf(ctx, "$ git %s", strings.Join(args, " "))
	err := cmd.Run()
	if text := strings.TrimSpace(out.String()); text != "" {
		logf(ctx, "%s", text)
	}
	if err != nil {
		logf(ctx, "[exit] %v", err)
	}
	if cctx.Err() == context.DeadlineExceeded {
		return out.Bytes(), syncErr("TIMEOUT", "Git 操作超时", true)
	}
	return out.Bytes(), err
}

func runIn(ctx context.Context, timeout time.Duration, dir string, args ...string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/false", "LC_ALL=C")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	logf(ctx, "$ git %s", strings.Join(args, " "))
	err := cmd.Run()
	if text := strings.TrimSpace(out.String()); text != "" {
		logf(ctx, "%s", text)
	}
	if err != nil {
		logf(ctx, "[exit] %v", err)
	}
	if cctx.Err() == context.DeadlineExceeded {
		return out.Bytes(), syncErr("TIMEOUT", "Git 操作超时", true)
	}
	return out.Bytes(), err
}

// ValidateRemote checks that the remote is reachable and public.
func (s *Service) ValidateRemote(ctx context.Context, url string) error {
	out, err := run(ctx, 20*time.Second, "ls-remote", "--exit-code", url, "HEAD")
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = "无法访问仓库，可能不存在或不是公开仓库"
		}
		return syncErr("UNREACHABLE", msg, true)
	}
	return nil
}

// CloneMirror clones a bare mirror into a temporary directory then moves it into place.
func (s *Service) CloneMirror(ctx context.Context, owner, name, url string) (string, error) {
	finalPath := s.mirrorPath(owner, name)
	if _, err := os.Stat(finalPath); err == nil {
		return "", syncErr("ALREADY_EXISTS", "本地镜像已存在", false)
	}
	if err := os.MkdirAll(s.BaseDir, 0o755); err != nil {
		return "", syncErr("IO", err.Error(), false)
	}
	tmp := fmt.Sprintf("%s.tmp-%d", finalPath, time.Now().UnixNano())
	_ = os.RemoveAll(tmp)
	out, err := run(ctx, 10*time.Minute, "clone", "--mirror", "--quiet", url, tmp)
	if err != nil {
		_ = os.RemoveAll(tmp)
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = "克隆仓库失败"
		}
		return "", syncErr("CLONE_FAILED", msg, true)
	}
	if err := os.Rename(tmp, finalPath); err != nil {
		_ = os.RemoveAll(tmp)
		return "", syncErr("IO", err.Error(), false)
	}
	return finalPath, nil
}

func (s *Service) Exists(owner, name string) bool {
	_, err := os.Stat(s.mirrorPath(owner, name))
	return err == nil
}

func (s *Service) MirrorPath(owner, name string) string { return s.mirrorPath(owner, name) }

func (s *Service) RemoveMirror(owner, name string) error {
	return os.RemoveAll(s.mirrorPath(owner, name))
}

// DefaultBranch returns the local HEAD branch name for a bare mirror.
func (s *Service) DefaultBranch(ctx context.Context, owner, name string) string {
	p := s.mirrorPath(owner, name)
	out, err := run(ctx, 10*time.Second, "--git-dir", p, "symbolic-ref", "--short", "HEAD")
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return "main"
}

// RemoteDefaultBranch asks origin for its default branch.
func (s *Service) RemoteDefaultBranch(ctx context.Context, owner, name string) string {
	p := s.mirrorPath(owner, name)
	out, err := run(ctx, 20*time.Second, "--git-dir", p, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "ref: refs/heads/") && strings.HasSuffix(line, "\tHEAD") {
			return strings.TrimPrefix(strings.TrimSuffix(line, "\tHEAD"), "ref: refs/heads/")
		}
	}
	return ""
}

func refSnapshot(ctx context.Context, p string) (map[string]string, error) {
	out, err := run(ctx, 30*time.Second, "--git-dir", p, "for-each-ref", "--format=%(refname)%00%(objectname)", "refs/heads", "refs/tags")
	if err != nil {
		return nil, syncErr("GIT_READ_FAILED", strings.TrimSpace(string(out)), true)
	}
	refs := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) == 2 {
			refs[parts[0]] = parts[1]
		}
	}
	return refs, nil
}

func incomingSnapshot(ctx context.Context, p string) (map[string]string, error) {
	out, err := run(ctx, 30*time.Second, "--git-dir", p, "for-each-ref", "--format=%(refname)%00%(objectname)", "refs/repoharbor/incoming")
	if err != nil {
		return nil, syncErr("GIT_READ_FAILED", strings.TrimSpace(string(out)), true)
	}
	refs := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) == 2 {
			refs[parts[0]] = parts[1]
		}
	}
	return refs, nil
}

func isAncestor(ctx context.Context, p, old, new string) bool {
	_, err := run(ctx, 30*time.Second, "--git-dir", p, "merge-base", "--is-ancestor", old, new)
	return err == nil
}

func cleanupIncoming(ctx context.Context, p string) {
	out, err := run(ctx, 20*time.Second, "--git-dir", p, "for-each-ref", "--format=%(refname)", "refs/repoharbor/incoming")
	if err != nil {
		return
	}
	for _, ref := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if ref != "" {
			_, _ = run(ctx, 10*time.Second, "--git-dir", p, "update-ref", "-d", ref)
		}
	}
}

// UpdateMirror fetches remote refs into a staging namespace, verifies that no
// branch was rewritten or deleted, then applies the safe updates.
func (s *Service) UpdateMirror(ctx context.Context, owner, name string) (string, error) {
	p := s.mirrorPath(owner, name)
	if _, err := os.Stat(p); err != nil {
		return "", syncErr("MIRROR_MISSING", "本地镜像不存在", false)
	}
	oldRefs, err := refSnapshot(ctx, p)
	if err != nil {
		return "", err
	}
	cleanupIncoming(ctx, p)
	out, err := run(ctx, 10*time.Minute, "--git-dir", p, "fetch", "--quiet", "--prune", "--refmap=", "origin",
		"+refs/heads/*:refs/repoharbor/incoming/heads/*",
		"+refs/tags/*:refs/repoharbor/incoming/tags/*")
	if err != nil {
		cleanupIncoming(ctx, p)
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = "拉取远端更新失败"
		}
		return "", syncErr("FETCH_FAILED", msg, true)
	}
	incoming, err := incomingSnapshot(ctx, p)
	if err != nil {
		cleanupIncoming(ctx, p)
		return "", err
	}
	// Build incoming maps from the staging namespace.
	incHeads := map[string]string{}
	incTags := map[string]string{}
	for ref, sha := range incoming {
		switch {
		case strings.HasPrefix(ref, "refs/repoharbor/incoming/heads/"):
			incHeads["refs/heads/"+strings.TrimPrefix(ref, "refs/repoharbor/incoming/heads/")] = sha
		case strings.HasPrefix(ref, "refs/repoharbor/incoming/tags/"):
			incTags["refs/tags/"+strings.TrimPrefix(ref, "refs/repoharbor/incoming/tags/")] = sha
		}
	}
	// Reject branch deletions and non-fast-forward branch updates.
	for ref, oldSHA := range oldRefs {
		if !strings.HasPrefix(ref, "refs/heads/") {
			continue
		}
		newSHA, ok := incHeads[ref]
		if !ok {
			cleanupIncoming(ctx, p)
			return "", syncErr("REMOTE_REWRITTEN", "远端删除了分支 "+strings.TrimPrefix(ref, "refs/heads/")+"，已保留本地镜像", false)
		}
		if oldSHA != newSHA && !isAncestor(ctx, p, oldSHA, newSHA) {
			cleanupIncoming(ctx, p)
			return "", syncErr("REMOTE_REWRITTEN", "远端历史被改写（"+strings.TrimPrefix(ref, "refs/heads/")+"），已保留本地镜像", false)
		}
	}
	// Apply branch refs.
	for ref, newSHA := range incHeads {
		oldSHA, ok := oldRefs[ref]
		if ok && oldSHA == newSHA {
			continue
		}
		if _, err := run(ctx, 20*time.Second, "--git-dir", p, "update-ref", ref, newSHA); err != nil {
			cleanupIncoming(ctx, p)
			return "", syncErr("REF_UPDATE_FAILED", "更新本地分支失败", false)
		}
	}
	// Apply tags; tag updates/deletions are allowed and are not branch history rewrites.
	for ref, newSHA := range incTags {
		oldSHA, ok := oldRefs[ref]
		if ok && oldSHA == newSHA {
			continue
		}
		if _, err := run(ctx, 20*time.Second, "--git-dir", p, "update-ref", ref, newSHA); err != nil {
			cleanupIncoming(ctx, p)
			return "", syncErr("REF_UPDATE_FAILED", "更新本地标签失败", false)
		}
	}
	for ref := range oldRefs {
		if !strings.HasPrefix(ref, "refs/tags/") {
			continue
		}
		if _, ok := incTags[ref]; !ok {
			_, _ = run(ctx, 20*time.Second, "--git-dir", p, "update-ref", "-d", ref)
		}
	}
	cleanupIncoming(ctx, p)
	if branch := s.RemoteDefaultBranch(ctx, owner, name); branch != "" {
		_, _ = run(ctx, 10*time.Second, "--git-dir", p, "symbolic-ref", "HEAD", "refs/heads/"+branch)
	}
	return p, nil
}

// MirrorSize returns the on-disk size of a mirror.
func (s *Service) MirrorSize(owner, name string) int64 {
	var total int64
	_ = filepath.Walk(s.mirrorPath(owner, name), func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// Ref is a branch or tag.
type Ref struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Commit string `json:"commit"`
}

func (s *Service) Refs(ctx context.Context, owner, name string) ([]Ref, string, error) {
	p := s.mirrorPath(owner, name)
	out, err := run(ctx, 30*time.Second, "--git-dir", p, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(refname:short)", "refs/heads", "refs/tags")
	if err != nil {
		return nil, "", syncErr("GIT_READ_FAILED", strings.TrimSpace(string(out)), true)
	}
	refs := []Ref{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 3)
		if len(parts) != 3 {
			continue
		}
		typ := "branch"
		if strings.HasPrefix(parts[0], "refs/tags/") {
			typ = "tag"
		}
		refs = append(refs, Ref{Name: parts[2], Type: typ, Commit: parts[1]})
	}
	return refs, s.DefaultBranch(ctx, owner, name), nil
}

func cleanPath(p string) (string, error) {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	clean := path.Clean(p)
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return "", syncErr("INVALID_PATH", "路径不合法", false)
	}
	return clean, nil
}

func safeRef(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "-") || strings.Contains(ref, "..") || !refPattern.MatchString(ref) {
		return syncErr("INVALID_REF", "分支或标签不合法", false)
	}
	return nil
}

// TreeEntry describes one directory entry.
type TreeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
	Mode string `json:"mode"`
	Size int64  `json:"size"`
}

func (s *Service) Tree(ctx context.Context, owner, name, ref, dir string) ([]TreeEntry, error) {
	if err := safeRef(ref); err != nil {
		return nil, err
	}
	dir, err := cleanPath(dir)
	if err != nil {
		return nil, err
	}
	p := s.mirrorPath(owner, name)
	args := []string{"--git-dir", p, "ls-tree", "-l"}
	isTree := false
	if dir == "" {
		args = append(args, ref)
	} else {
		typeOut, typeErr := run(ctx, 20*time.Second, "--git-dir", p, "cat-file", "-t", ref+":"+dir)
		if typeErr != nil {
			return nil, syncErr("TREE_FAILED", "路径不存在", false)
		}
		if strings.TrimSpace(string(typeOut)) == "tree" {
			isTree = true
			args = append(args, ref+":"+dir)
		} else {
			args = append(args, ref, "--", dir)
		}
	}
	out, err := run(ctx, 30*time.Second, args...)
	if err != nil {
		return nil, syncErr("TREE_FAILED", strings.TrimSpace(string(out)), false)
	}
	entries := []TreeEntry{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		meta, name, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) < 4 {
			continue
		}
		size := int64(0)
		if fields[3] != "-" {
			size, _ = strconv.ParseInt(fields[3], 10, 64)
		}
		typ := fields[1]
		if typ == "tree" {
			typ = "dir"
		}
		entryPath := name
		if isTree {
			entryPath = path.Join(dir, name)
		}
		entries = append(entries, TreeEntry{Name: name, Path: entryPath, Type: typ, Mode: fields[0], Size: size})
	}
	sortTreeEntries(entries)
	return entries, nil
}

// sortTreeEntries presents directories before files and compares names
// case-insensitively. The original name is the final tie-breaker so entries
// that differ only by case keep a deterministic order.
func sortTreeEntries(entries []TreeEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		leftDir, rightDir := left.Type == "dir", right.Type == "dir"
		if leftDir != rightDir {
			return leftDir
		}
		leftName, rightName := strings.ToLower(left.Name), strings.ToLower(right.Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return left.Name < right.Name
	})
}

// FileContent is a text or binary file result.
type FileContent struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Binary    bool   `json:"binary"`
	TooLarge  bool   `json:"too_large"`
	LFS       bool   `json:"lfs"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

func (s *Service) File(ctx context.Context, owner, name, ref, filePath string) (FileContent, error) {
	if err := safeRef(ref); err != nil {
		return FileContent{}, err
	}
	clean, err := cleanPath(filePath)
	if err != nil {
		return FileContent{}, err
	}
	if clean == "" {
		return FileContent{}, syncErr("INVALID_PATH", "文件路径不能为空", false)
	}
	p := s.mirrorPath(owner, name)
	sizeOut, err := run(ctx, 20*time.Second, "--git-dir", p, "cat-file", "-s", ref+":"+clean)
	if err != nil {
		return FileContent{}, syncErr("FILE_NOT_FOUND", "文件不存在", false)
	}
	size, _ := strconv.ParseInt(strings.TrimSpace(string(sizeOut)), 10, 64)
	res := FileContent{Path: clean, Size: size}
	if size > maxInlineFile {
		res.TooLarge = true
		return res, nil
	}
	data, err := run(ctx, 30*time.Second, "--git-dir", p, "show", ref+":"+clean)
	if err != nil {
		return FileContent{}, syncErr("FILE_READ_FAILED", "读取文件失败", false)
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		res.Binary = true
		return res, nil
	}
	content := string(data)
	if strings.HasPrefix(content, "version https://git-lfs.github.com/spec/v1") {
		res.LFS = true
	}
	res.Content = content
	return res, nil
}

// Readme returns the first README candidate.
func (s *Service) Readme(ctx context.Context, owner, name, ref string) (FileContent, error) {
	for _, candidate := range []string{"README.md", "README.markdown", "README", "readme.md"} {
		f, err := s.File(ctx, owner, name, ref, candidate)
		if err == nil {
			return f, nil
		}
	}
	return FileContent{}, syncErr("README_NOT_FOUND", "README 不存在", false)
}

// Commit is one commit list item.
type Commit struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Email   string `json:"email"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

func (s *Service) Commits(ctx context.Context, owner, name, ref string, offset, limit int) ([]Commit, bool, error) {
	if err := safeRef(ref); err != nil {
		return nil, false, err
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	p := s.mirrorPath(owner, name)
	out, err := run(ctx, 60*time.Second, "--git-dir", p, "log",
		fmt.Sprintf("--skip=%d", offset), fmt.Sprintf("--max-count=%d", limit+1),
		"--format=%H%x00%an%x00%ae%x00%aI%x00%s", ref)
	if err != nil {
		return nil, false, syncErr("LOG_FAILED", strings.TrimSpace(string(out)), false)
	}
	commits := []Commit{}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 5)
		if len(parts) != 5 {
			continue
		}
		commits = append(commits, Commit{SHA: parts[0], Author: parts[1], Email: parts[2], Date: parts[3], Subject: parts[4]})
	}
	hasMore := false
	if len(commits) > limit {
		commits = commits[:limit]
		hasMore = true
	}
	return commits, hasMore, nil
}

// CommitDetail is a commit with changed files and a unified diff.
type CommitDetail struct {
	Commit
	Body          string     `json:"body"`
	Files         []FileStat `json:"files"`
	Diff          string     `json:"diff"`
	DiffTruncated bool       `json:"diff_truncated"`
}

type FileStat struct {
	Path    string `json:"path"`
	Added   int64  `json:"added"`
	Deleted int64  `json:"deleted"`
}

func (s *Service) CommitDetail(ctx context.Context, owner, name, sha string) (CommitDetail, error) {
	if !regexp.MustCompile(`^[0-9a-fA-F]{4,64}$`).MatchString(sha) {
		return CommitDetail{}, syncErr("INVALID_SHA", "提交 ID 不合法", false)
	}
	p := s.mirrorPath(owner, name)
	meta, err := run(ctx, 30*time.Second, "--git-dir", p, "show", "-s", "--format=%H%x00%an%x00%ae%x00%aI%x00%s%x00%b", sha)
	if err != nil {
		return CommitDetail{}, syncErr("COMMIT_NOT_FOUND", "提交不存在", false)
	}
	parts := strings.SplitN(strings.TrimRight(string(meta), "\n"), "\x00", 6)
	if len(parts) < 5 {
		return CommitDetail{}, syncErr("COMMIT_READ_FAILED", "读取提交信息失败", false)
	}
	detail := CommitDetail{Commit: Commit{SHA: parts[0], Author: parts[1], Email: parts[2], Date: parts[3], Subject: parts[4]}}
	if len(parts) == 6 {
		detail.Body = parts[5]
	}
	stat, err := run(ctx, 30*time.Second, "--git-dir", p, "show", "--numstat", "--format=", sha)
	if err == nil {
		for _, line := range strings.Split(string(stat), "\n") {
			fields := strings.SplitN(line, "\t", 3)
			if len(fields) != 3 {
				continue
			}
			added, _ := strconv.ParseInt(fields[0], 10, 64)
			deleted, _ := strconv.ParseInt(fields[1], 10, 64)
			detail.Files = append(detail.Files, FileStat{Path: fields[2], Added: added, Deleted: deleted})
		}
	}
	// Diff against the first parent. Merge commits otherwise produce no
	// useful patch with plain `git show`.
	parentsOut, _ := run(ctx, 20*time.Second, "--git-dir", p, "rev-list", "--parents", "-n", "1", sha)
	parentFields := strings.Fields(string(parentsOut))
	diffArgs := []string{"--git-dir", p}
	if len(parentFields) >= 2 {
		diffArgs = append(diffArgs, "diff", "--find-renames", "--no-color", parentFields[1], sha)
	} else {
		diffArgs = append(diffArgs, "show", "--format=", "--patch", "--root", "--no-color", sha)
	}
	diff, err := run(ctx, 60*time.Second, diffArgs...)
	if err != nil {
		return detail, nil
	}
	if len(diff) > maxInlineFile {
		detail.Diff = string(diff[:maxInlineFile])
		detail.DiffTruncated = true
	} else {
		detail.Diff = string(diff)
	}
	return detail, nil
}

// RawFile returns file bytes for downloads. It refuses files above 50 MiB.
func (s *Service) RawFile(ctx context.Context, owner, name, ref, filePath string) ([]byte, error) {
	if err := safeRef(ref); err != nil {
		return nil, err
	}
	clean, err := cleanPath(filePath)
	if err != nil {
		return nil, err
	}
	p := s.mirrorPath(owner, name)
	sizeOut, err := run(ctx, 20*time.Second, "--git-dir", p, "cat-file", "-s", ref+":"+clean)
	if err != nil {
		return nil, syncErr("FILE_NOT_FOUND", "文件不存在", false)
	}
	size, _ := strconv.ParseInt(strings.TrimSpace(string(sizeOut)), 10, 64)
	if size > 50*1024*1024 {
		return nil, syncErr("FILE_TOO_LARGE", "文件超过 50 MiB，暂不支持下载", false)
	}
	return run(ctx, 60*time.Second, "--git-dir", p, "show", ref+":"+clean)
}
