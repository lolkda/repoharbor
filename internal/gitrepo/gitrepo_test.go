package gitrepo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func setupRemote(t *testing.T) (remote, work string) {
	t.Helper()
	root := t.TempDir()
	remote = filepath.Join(root, "remote.git")
	work = filepath.Join(root, "work")
	git(t, root, "init", "--bare", "--initial-branch=main", remote)
	git(t, root, "clone", remote, work)
	git(t, work, "config", "user.name", "Test")
	git(t, work, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(work, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "dir", "file.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, work, "add", ".")
	git(t, work, "commit", "-m", "initial")
	git(t, work, "push", "origin", "main")
	return remote, work
}

func TestMirrorReadAndUpdate(t *testing.T) {
	ctx := context.Background()
	remote, work := setupRemote(t)
	svc := New(filepath.Join(t.TempDir(), "repos"))
	if err := svc.ValidateRemote(ctx, remote); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CloneMirror(ctx, "owner", "repo", remote); err != nil {
		t.Fatal(err)
	}
	refs, def, err := svc.Refs(ctx, "owner", "repo")
	if err != nil || len(refs) == 0 || def != "main" {
		t.Fatalf("refs=%v def=%q err=%v", refs, def, err)
	}
	entries, err := svc.Tree(ctx, "owner", "repo", "main", "dir")
	if err != nil || len(entries) != 1 || entries[0].Path != "dir/file.txt" || entries[0].Type != "blob" {
		t.Fatalf("tree=%+v err=%v", entries, err)
	}
	file, err := svc.File(ctx, "owner", "repo", "main", "dir/file.txt")
	if err != nil || file.Content != "one\n" || file.Binary {
		t.Fatalf("file=%+v err=%v", file, err)
	}
	commits, more, err := svc.Commits(ctx, "owner", "repo", "main", 0, 10)
	if err != nil || len(commits) != 1 || more {
		t.Fatalf("commits=%+v more=%v err=%v", commits, more, err)
	}
	if err := os.WriteFile(filepath.Join(work, "dir", "file.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, work, "add", ".")
	git(t, work, "commit", "-m", "second")
	git(t, work, "push", "origin", "main")
	if _, err := svc.UpdateMirror(ctx, "owner", "repo"); err != nil {
		t.Fatal(err)
	}
	commits, _, err = svc.Commits(ctx, "owner", "repo", "main", 0, 10)
	if err != nil || len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %+v err=%v", commits, err)
	}
}

func TestTreeOrdersDirectoriesThenNamesCaseInsensitively(t *testing.T) {
	ctx := context.Background()
	remote, work := setupRemote(t)

	for _, dir := range []string{"apples", "beta", "Zoo"} {
		dirPath := filepath.Join(work, dir)
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dirPath, "keep.txt"), []byte("keep\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"alpha.txt", "Bravo.txt", "When.txt"} {
		if err := os.WriteFile(filepath.Join(work, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, work, "add", ".")
	git(t, work, "commit", "-m", "add sort fixtures")
	git(t, work, "push", "origin", "main")

	svc := New(filepath.Join(t.TempDir(), "repos"))
	if _, err := svc.CloneMirror(ctx, "owner", "repo", remote); err != nil {
		t.Fatal(err)
	}
	entries, err := svc.Tree(ctx, "owner", "repo", "main", "")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name
	}
	want := []string{"apples", "beta", "dir", "Zoo", "alpha.txt", "Bravo.txt", "README.md", "When.txt"}
	if !slices.Equal(names, want) {
		t.Fatalf("tree order = %v, want %v", names, want)
	}
}

func TestUpdateMirrorRejectsForcePush(t *testing.T) {
	ctx := context.Background()
	remote, work := setupRemote(t)
	svc := New(filepath.Join(t.TempDir(), "repos"))
	if _, err := svc.CloneMirror(ctx, "owner", "repo", remote); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, work, "add", ".")
	git(t, work, "commit", "-m", "second")
	git(t, work, "push", "origin", "main")
	if _, err := svc.UpdateMirror(ctx, "owner", "repo"); err != nil {
		t.Fatal(err)
	}
	git(t, work, "reset", "--hard", "HEAD~1")
	git(t, work, "push", "--force", "origin", "main")
	_, err := svc.UpdateMirror(ctx, "owner", "repo")
	if err == nil {
		t.Fatal("expected force push to be rejected")
	}
	var se *SyncError
	if !errors.As(err, &se) || se.Code != "REMOTE_REWRITTEN" {
		t.Fatalf("expected REMOTE_REWRITTEN, got %#v", err)
	}
	commits, _, err := svc.Commits(ctx, "owner", "repo", "main", 0, 10)
	if err != nil || len(commits) != 2 {
		t.Fatalf("local mirror should be preserved with 2 commits, got %+v err=%v", commits, err)
	}
}

func TestParseURL(t *testing.T) {
	owner, name, normalized, err := ParseURL("https://github.com/octocat/Hello-World")
	if err != nil || owner != "octocat" || name != "Hello-World" || normalized != "https://github.com/octocat/Hello-World" {
		t.Fatalf("got %q %q %q %v", owner, name, normalized, err)
	}
	for _, bad := range []string{"", "http://github.com/a/b", "https://gitlab.com/a/b", "https://github.com/a/b/tree/main", "https://github.com/a"} {
		if _, _, _, err := ParseURL(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}
