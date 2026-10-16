// Package security provides shared hardening helpers (rate limiting, crypto).
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// ─── Simple IP rate limiter (login brute-force protection) ───────────────────

type rateEntry struct {
	count    int
	windowAt time.Time
}

// RateLimiter is a fixed-window in-memory limiter keyed by client IP.
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*rateEntry
}

// NewRateLimiter creates a limiter allowing `limit` hits per `window`.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		limit:   limit,
		window:  window,
		entries: map[string]*rateEntry{},
	}
}

// Allow reports whether the key is within the rate limit.
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e := l.entries[key]
	if e == nil || now.Sub(e.windowAt) >= l.window {
		l.entries[key] = &rateEntry{count: 1, windowAt: now}
		return true
	}
	if e.count >= l.limit {
		return false
	}
	e.count++
	return true
}

// Middleware wraps a handler with IP-based rate limiting.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		if !l.Allow(ip) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(l.window.Seconds())))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error_code":"RATE_LIMITED","message":"Too many requests"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── Credential encryption at rest (AES-256-GCM) ────────────────────────────

// Encryptor encrypts/decrypts credential input blobs.
type Encryptor struct {
	aead cipher.AEAD
}

// NewEncryptor derives a key from the provided secret.
func NewEncryptor(secret string) (*Encryptor, error) {
	if secret == "" {
		return nil, fmt.Errorf("encryption secret is required")
	}
	sum := sha256.Sum256([]byte("aiwx-cred-v1:" + secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Encryptor{aead: aead}, nil
}

// Encrypt returns base64(nonce||ciphertext).
func (e *Encryptor) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := e.aead.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt.
func (e *Encryptor) Decrypt(encoded string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	ns := e.aead.NonceSize()
	if len(raw) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return e.aead.Open(nil, raw[:ns], raw[ns:], nil)
}
