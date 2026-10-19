package sync

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResolveSCMAuth_NilIsNone(t *testing.T) {
	auth, err := ResolveSCMAuth(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Kind != SCMAuthNone {
		t.Fatalf("kind = %q, want none", auth.Kind)
	}
}

func TestResolveSCMAuth_GitSSHKey(t *testing.T) {
	auth, err := ResolveSCMAuth(context.Background(), &Credential{
		Name:   "ssh",
		Type:   "Git SSH Key",
		Inputs: map[string]string{"ssh_key": "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Kind != SCMAuthSSHKey {
		t.Fatalf("kind = %q, want ssh_key", auth.Kind)
	}
	if !strings.Contains(auth.SSHKey, "PRIVATE KEY") {
		t.Fatalf("ssh key not loaded")
	}
}

func TestResolveSCMAuth_GitHubPAT(t *testing.T) {
	auth, err := ResolveSCMAuth(context.Background(), &Credential{
		Name:   "pat",
		Type:   "GitHub Personal Access Token",
		Inputs: map[string]string{"token": "ghp_secret"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Kind != SCMAuthToken {
		t.Fatalf("kind = %q, want token", auth.Kind)
	}
	if auth.Username != "x-access-token" {
		t.Fatalf("username = %q", auth.Username)
	}
	if auth.Token != "ghp_secret" {
		t.Fatalf("token not set")
	}
}

func TestResolveSCMAuth_UsernamePassword(t *testing.T) {
	auth, err := ResolveSCMAuth(context.Background(), &Credential{
		Name:   "up",
		Type:   "Username/Password",
		Inputs: map[string]string{"username": "bot", "password": "pw"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Kind != SCMAuthUserPass || auth.Username != "bot" || auth.Password != "pw" {
		t.Fatalf("unexpected auth: %+v", auth)
	}
}

func TestResolveSCMAuth_GitHubAppMintsToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/app/installations/123/access_tokens") {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"ghs_install_token","expires_at":"2030-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	auth, err := ResolveSCMAuth(context.Background(), &Credential{
		Name: "app",
		Type: "GitHub App",
		Inputs: map[string]string{
			"app_id":          "1",
			"installation_id": "123",
			"private_key":     pemKey,
			"base_url":        srv.URL,
		},
	}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if auth.Kind != SCMAuthGitHubApp {
		t.Fatalf("kind = %q", auth.Kind)
	}
	if auth.Token != "ghs_install_token" {
		t.Fatalf("token = %q", auth.Token)
	}
	if auth.Username != "x-access-token" {
		t.Fatalf("username = %q", auth.Username)
	}
}

func TestSignGitHubAppJWT_Shape(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))

	jwt, err := signGitHubAppJWT("42", pemKey, time.Unix(1700000000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt parts = %d", len(parts))
	}
}

func TestRedactGitOutput(t *testing.T) {
	in := "fatal: repository 'https://user:secretpass@github.com/org/repo.git/' not found"
	out := redactGitOutput(in)
	if strings.Contains(out, "secretpass") {
		t.Fatalf("secret leaked: %s", out)
	}
}
