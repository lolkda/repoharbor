package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"repoharbor/internal/config"
	"repoharbor/internal/events"
	"repoharbor/internal/gitrepo"
	"repoharbor/internal/schedule"
	"repoharbor/internal/store"
	db "repoharbor/internal/store/db"
)

var ErrBusy = errors.New("repository is already syncing")

// Scheduler runs due and manual mirror synchronizations.
type Scheduler struct {
	cfg    config.Config
	store  *store.Store
	git    *gitrepo.Service
	broker *events.Broker
	sem    chan struct{}

	mu    sync.Mutex
	locks map[int64]*sync.Mutex
}

func New(cfg config.Config, st *store.Store, git *gitrepo.Service, broker *events.Broker) *Scheduler {
	return &Scheduler{
		cfg:    cfg,
		store:  st,
		git:    git,
		broker: broker,
		sem:    make(chan struct{}, 3),
		locks:  make(map[int64]*sync.Mutex),
	}
}

func (s *Scheduler) repoLock(id int64) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[id] == nil {
		s.locks[id] = &sync.Mutex{}
	}
	return s.locks[id]
}

// Start recovers interrupted work and launches the periodic due scanner.
func (s *Scheduler) Start(ctx context.Context) {
	now := store.Now()
	_ = s.store.Q.MarkSyncingRepositoriesFailed(ctx, now)
	_ = s.store.Q.MarkRunningSyncRunsFailed(ctx, now)
	repos, err := s.store.Q.ListEnabledRepositories(ctx)
	if err == nil {
		nowTime := time.Now()
		for _, repo := range repos {
			next, nextErr := schedule.Next(repo.CronExpression, nowTime)
			if nextErr != nil {
				slog.Warn("invalid repository schedule", "repository_id", repo.ID, "cron", repo.CronExpression, "error", nextErr)
				continue
			}
			_ = s.store.Q.UpdateRepositoryNextSync(ctx, db.UpdateRepositoryNextSyncParams{
				NextSyncAt: next.UTC().Format(time.RFC3339),
				UpdatedAt:  now,
				ID:         repo.ID,
			})
		}
	}
	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.scanDue(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scanDue(ctx)
		}
	}
}

func (s *Scheduler) scanDue(ctx context.Context) {
	repos, err := s.store.Q.ListDueRepositories(ctx, store.Now())
	if err != nil {
		return
	}
	for _, repo := range repos {
		repo := repo
		go s.runWithRetries(context.Background(), repo, "schedule", 1)
	}
}

// Trigger queues an immediate sync and returns before the Git work starts.
func (s *Scheduler) Trigger(ctx context.Context, repoID int64, trigger string) error {
	repo, err := s.store.Q.GetRepository(ctx, repoID)
	if err != nil {
		return err
	}
	if repo.Status == "syncing" {
		return ErrBusy
	}
	go s.runWithRetries(context.Background(), repo, trigger, 1)
	return nil
}

// ForceReset removes the local mirror and queues a fresh clone.
func (s *Scheduler) ForceReset(ctx context.Context, repoID int64) error {
	repo, err := s.store.Q.GetRepository(ctx, repoID)
	if err != nil {
		return err
	}
	if repo.Status == "syncing" {
		return ErrBusy
	}
	if err := s.git.RemoveMirror(repo.Owner, repo.Name); err != nil {
		return err
	}
	_ = s.store.Q.UpdateRepositoryStatus(ctx, db.UpdateRepositoryStatusParams{
		Status: "pending", LastErrorCode: "", LastErrorMessage: "", UpdatedAt: store.Now(), ID: repo.ID,
	})
	go s.runWithRetries(context.Background(), repo, "force_reset", 1)
	return nil
}

func (s *Scheduler) runWithRetries(ctx context.Context, repo db.Repository, trigger string, attempt int) {
	lock := s.repoLock(repo.ID)
	if !lock.TryLock() {
		return
	}
	defer lock.Unlock()

	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	current, err := s.store.Q.GetRepository(ctx, repo.ID)
	if err != nil {
		return
	}
	started := time.Now()
	run, err := s.store.Q.CreateSyncRun(ctx, db.CreateSyncRunParams{
		RepositoryID: current.ID,
		TriggerType:  trigger,
		Status:       "running",
		Attempt:      int64(attempt),
		StartedAt:    started.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	_ = s.store.Q.UpdateRepositoryStatus(ctx, db.UpdateRepositoryStatusParams{
		Status: "syncing", LastErrorCode: "", LastErrorMessage: "", UpdatedAt: store.Now(), ID: current.ID,
	})
	s.broker.Publish(events.Event{Type: "sync.started", Data: map[string]any{"repository_id": current.ID, "trigger": trigger, "attempt": attempt}})

	logPath := filepath.Join(s.cfg.LogsDir, strconv.FormatInt(current.ID, 10), fmt.Sprintf("%s-%d.log", time.Now().UTC().Format("20060102T150405"), run.ID))
	logFile, logErr := createLogFile(logPath)
	syncCtx := ctx
	if logErr == nil {
		defer logFile.Close()
		syncCtx = gitrepo.WithLog(ctx, logFile)
	}
	syncErr := s.syncOnce(syncCtx, current)
	finished := time.Now()
	duration := finished.Sub(started).Milliseconds()
	logExcerpt := readLogExcerpt(logPath, 16*1024)
	nextSync := ""
	if current.Enabled == 1 {
		if next, nextErr := schedule.Next(current.CronExpression, finished); nextErr == nil {
			nextSync = next.UTC().Format(time.RFC3339)
		} else {
			slog.Warn("invalid repository schedule", "repository_id", current.ID, "cron", current.CronExpression, "error", nextErr)
		}
	}

	if syncErr == nil {
		branch := s.git.DefaultBranch(ctx, current.Owner, current.Name)
		size := s.git.MirrorSize(current.Owner, current.Name)
		_ = s.store.Q.UpdateRepositoryAfterSync(ctx, db.UpdateRepositoryAfterSyncParams{
			Status: "success", DefaultBranch: branch,
			LastSyncAt: finished.UTC().Format(time.RFC3339), NextSyncAt: nextSync,
			LastErrorCode: "", LastErrorMessage: "", SizeBytes: size,
			UpdatedAt: store.Now(), ID: current.ID,
		})
		_ = s.store.Q.FinishSyncRun(ctx, db.FinishSyncRunParams{
			Status: "success", FinishedAt: finished.UTC().Format(time.RFC3339), DurationMs: duration,
			ErrorCode: "", ErrorSummary: "", LogExcerpt: logExcerpt, ID: run.ID,
		})
		s.broker.Publish(events.Event{Type: "sync.finished", Data: map[string]any{"repository_id": current.ID, "status": "success", "duration_ms": duration}})
		s.cleanupLogs(current.ID)
		return
	}

	code := "SYNC_FAILED"
	message := syncErr.Error()
	retryable := false
	var se *gitrepo.SyncError
	if errors.As(syncErr, &se) {
		code = se.Code
		message = se.Message
		retryable = se.Retryable
	}
	message = truncate(message, 4000)
	_ = s.store.Q.UpdateRepositoryStatus(ctx, db.UpdateRepositoryStatusParams{
		Status: "failed", LastErrorCode: code, LastErrorMessage: message, UpdatedAt: store.Now(), ID: current.ID,
	})
	_ = s.store.Q.UpdateRepositoryNextSync(ctx, db.UpdateRepositoryNextSyncParams{
		NextSyncAt: nextSync, UpdatedAt: store.Now(), ID: current.ID,
	})
	_ = s.store.Q.FinishSyncRun(ctx, db.FinishSyncRunParams{
		Status: "failed", FinishedAt: finished.UTC().Format(time.RFC3339), DurationMs: duration,
		ErrorCode: code, ErrorSummary: message, LogExcerpt: logExcerpt, ID: run.ID,
	})
	s.cleanupLogs(current.ID)
	s.broker.Publish(events.Event{Type: "sync.failed", Data: map[string]any{"repository_id": current.ID, "code": code, "message": message}})

	if retryable && attempt < 1+s.cfg.MaxRetries {
		delay := retryDelay(attempt)
		time.AfterFunc(delay, func() {
			s.runWithRetries(context.Background(), current, "retry", attempt+1)
		})
	}
}

func (s *Scheduler) syncOnce(ctx context.Context, repo db.Repository) error {
	if !s.git.Exists(repo.Owner, repo.Name) {
		_, err := s.git.CloneMirror(ctx, repo.Owner, repo.Name, repo.Url)
		return err
	}
	_, err := s.git.UpdateMirror(ctx, repo.Owner, repo.Name)
	return err
}

func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 1 * time.Minute
	case 2:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func createLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.Create(path)
}

func readLogExcerpt(path string, max int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(data) > max {
		data = data[len(data)-max:]
	}
	return strings.TrimSpace(string(data))
}

// cleanupLogs keeps the newest 20 log files and removes files older than 30 days.
func (s *Scheduler) cleanupLogs(repoID int64) {
	dir := filepath.Join(s.cfg.LogsDir, strconv.FormatInt(repoID, 10))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	kept := 0
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		kept++
		if kept > 20 || info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}
