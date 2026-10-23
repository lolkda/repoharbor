package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	SessionCookie = "repoharbor_session"
	passwordMemory = 64 * 1024
	passwordTime   = 1
	passwordThreads = 4
	passwordKeyLen = 32
)

// HashPassword returns an Argon2id encoded hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, passwordKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		passwordMemory, passwordTime, passwordThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

// VerifyPassword compares a plaintext password with an encoded Argon2id hash.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory uint32
	var iterations uint32
	var threads uint8
	for _, item := range strings.Split(parts[3], ",") {
		kv := strings.SplitN(item, "=", 2)
		if len(kv) != 2 {
			return false
		}
		switch kv[0] {
		case "m":
			n, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return false
			}
			memory = uint32(n)
		case "t":
			n, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return false
			}
			iterations = uint32(n)
		case "p":
			n, err := strconv.ParseUint(kv[1], 10, 8)
			if err != nil {
				return false
			}
			threads = uint8(n)
		}
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(want, got) == 1
}

// NewSessionToken returns the raw cookie value and its database hash.
func NewSessionToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

// HashToken hashes a raw session token for at-rest storage.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// NewCSRFToken returns a random CSRF token.
func NewCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type loginAttempt struct {
	failures    int
	first       time.Time
	last        time.Time
	lockedUntil time.Time
}

// Limiter implements a small in-memory login rate limiter.
type Limiter struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

func NewLimiter() *Limiter { return &Limiter{attempts: make(map[string]*loginAttempt)} }

// Allow reports whether a login attempt is currently allowed.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.attempts[key]
	if !ok {
		return true, 0
	}
	now := time.Now()
	if now.Before(a.lockedUntil) {
		return false, time.Until(a.lockedUntil)
	}
	if now.Sub(a.first) > 15*time.Minute {
		delete(l.attempts, key)
		return true, 0
	}
	return true, 0
}

// Fail records a failed login and locks after five failures.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	a, ok := l.attempts[key]
	if !ok || now.Sub(a.first) > 15*time.Minute {
		l.attempts[key] = &loginAttempt{failures: 1, first: now, last: now}
		return
	}
	a.failures++
	a.last = now
	if a.failures >= 5 {
		a.lockedUntil = now.Add(15 * time.Minute)
	}
}

// Reset clears failures after a successful login.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

var ErrInvalidPassword = errors.New("invalid password")
