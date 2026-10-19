// Package sync implements project synchronization from Git repositories.
package sync

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SCMAuthKind identifies how git should authenticate.
type SCMAuthKind string

const (
	SCMAuthNone       SCMAuthKind = "none"
	SCMAuthSSHKey     SCMAuthKind = "ssh_key"
	SCMAuthUserPass   SCMAuthKind = "user_pass"
	SCMAuthToken      SCMAuthKind = "token"
	SCMAuthGitHubApp  SCMAuthKind = "github_app"
)

// SCMAuth is the resolved authentication material for a git remote.
// Secrets must never be logged.
type SCMAuth struct {
	Kind SCMAuthKind

	// HTTPS username/password (token auth uses Username + Token as password).
	Username string
	Password string
	Token    string

	// SSH
	SSHKey        string
	SSHKnownHosts string

	// GitHub App (minted at resolve time into Token/Username)
	GitHubAppID         string
	GitHubInstallationID string
	GitHubPrivateKeyPEM string
	GitHubBaseURL       string
}

// Credential is the decrypted credential used for SCM auth.
type Credential struct {
	ID    string
	Name  string
	Type  string // credential type name, e.g. "Git SSH Key"
	Inputs map[string]string
}

// ResolveSCMAuth maps a credential to git auth material.
// nil credential → SCMAuthNone (public clone).
func ResolveSCMAuth(ctx context.Context, cred *Credential, httpClient *http.Client) (*SCMAuth, error) {
	if cred == nil {
		return &SCMAuth{Kind: SCMAuthNone}, nil
	}

	switch normalizeTypeName(cred.Type) {
	case "git ssh key", "ssh key":
		key := firstNonEmpty(cred.Inputs, "ssh_key", "private_key")
		if key == "" {
			return nil, fmt.Errorf("credential %q: missing ssh_key", cred.Name)
		}
		return &SCMAuth{
			Kind:          SCMAuthSSHKey,
			SSHKey:        key,
			SSHKnownHosts: cred.Inputs["ssh_known_hosts"],
		}, nil

	case "source control", "scm", "username/password", "username_password":
		if key := firstNonEmpty(cred.Inputs, "ssh_key", "private_key"); key != "" {
			return &SCMAuth{
				Kind:          SCMAuthSSHKey,
				SSHKey:        key,
				SSHKnownHosts: cred.Inputs["ssh_known_hosts"],
			}, nil
		}
		user := firstNonEmpty(cred.Inputs, "username", "user")
		pass := firstNonEmpty(cred.Inputs, "password", "token", "secret")
		if pass == "" {
			return nil, fmt.Errorf("credential %q: missing password/token", cred.Name)
		}
		if user == "" {
			user = "git"
		}
		return &SCMAuth{Kind: SCMAuthUserPass, Username: user, Password: pass}, nil

	case "github personal access token", "github pat", "github token":
		user := firstNonEmpty(cred.Inputs, "username")
		if user == "" {
			user = "x-access-token"
		}
		token := firstNonEmpty(cred.Inputs, "token", "password", "personal_access_token", "access_token")
		if token == "" {
			return nil, fmt.Errorf("credential %q: missing token", cred.Name)
		}
		return &SCMAuth{Kind: SCMAuthToken, Username: user, Token: token, Password: token}, nil

	case "git personal access token", "git token", "personal access token":
		user := firstNonEmpty(cred.Inputs, "username")
		if user == "" {
			user = "git"
		}
		token := firstNonEmpty(cred.Inputs, "token", "password")
		if token == "" {
			return nil, fmt.Errorf("credential %q: missing token", cred.Name)
		}
		return &SCMAuth{Kind: SCMAuthToken, Username: user, Token: token, Password: token}, nil

	case "github app":
		auth := &SCMAuth{
			Kind:                 SCMAuthGitHubApp,
			GitHubAppID:          firstNonEmpty(cred.Inputs, "app_id", "github_app_id"),
			GitHubInstallationID: firstNonEmpty(cred.Inputs, "installation_id", "github_installation_id"),
			GitHubPrivateKeyPEM:  firstNonEmpty(cred.Inputs, "private_key", "github_private_key"),
			GitHubBaseURL:        firstNonEmpty(cred.Inputs, "base_url", "github_base_url", "api_url"),
		}
		if auth.GitHubAppID == "" || auth.GitHubInstallationID == "" || auth.GitHubPrivateKeyPEM == "" {
			return nil, fmt.Errorf("credential %q: github app requires app_id, installation_id, private_key", cred.Name)
		}
		if auth.GitHubBaseURL == "" {
			auth.GitHubBaseURL = "https://api.github.com"
		}
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
		token, err := MintGitHubInstallationToken(ctx, httpClient, auth)
		if err != nil {
			return nil, fmt.Errorf("mint github app token: %w", err)
		}
		auth.Username = "x-access-token"
		auth.Token = token
		auth.Password = token
		return auth, nil

	default:
		// Fallback: if inputs look like a token/password, use them.
		if key := firstNonEmpty(cred.Inputs, "ssh_key", "private_key"); key != "" {
			return &SCMAuth{Kind: SCMAuthSSHKey, SSHKey: key}, nil
		}
		if token := firstNonEmpty(cred.Inputs, "token", "password"); token != "" {
			user := firstNonEmpty(cred.Inputs, "username")
			if user == "" {
				user = "git"
			}
			return &SCMAuth{Kind: SCMAuthToken, Username: user, Token: token, Password: token}, nil
		}
		return nil, fmt.Errorf("credential %q (type %q) cannot be used for project sync", cred.Name, cred.Type)
	}
}

func normalizeTypeName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func firstNonEmpty(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(m[k]); v != "" {
			return v
		}
	}
	return ""
}

// ─── Git process environment ──────────────────────────────────────────────────

// gitEnv prepares environment and temporary files for an authenticated git run.
// cleanup must be deferred by the caller.
func (a *SCMAuth) gitEnv(ctx context.Context) (env []string, cleanup func(), err error) {
	cleanup = func() {}
	if a == nil || a.Kind == SCMAuthNone {
		return os.Environ(), cleanup, nil
	}

	tmpDir, err := os.MkdirTemp("", "aiwx-git-auth-")
	if err != nil {
		return nil, cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(tmpDir) }

	env = os.Environ()

	switch a.Kind {
	case SCMAuthSSHKey:
		keyPath := filepath.Join(tmpDir, "id_ssh")
		if err := os.WriteFile(keyPath, []byte(a.SSHKey+"\n"), 0o600); err != nil {
			cleanup()
			return nil, func() {}, err
		}
		// Ensure trailing newline and no wide permissions.
		_ = os.Chmod(keyPath, 0o600)

		knownHosts := filepath.Join(tmpDir, "known_hosts")
		if a.SSHKnownHosts != "" {
			_ = os.WriteFile(knownHosts, []byte(a.SSHKnownHosts+"\n"), 0o600)
		}

		sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=%s -o BatchMode=yes",
			keyPath, knownHosts)
		env = append(env,
			"GIT_SSH_COMMAND="+sshCmd,
			"GIT_TERMINAL_PROMPT=0",
		)

	case SCMAuthUserPass, SCMAuthToken:
		user := a.Username
		if user == "" {
			user = "git"
		}
		pass := a.Password
		if pass == "" {
			pass = a.Token
		}
		askpass := filepath.Join(tmpDir, "askpass.sh")
		script := "#!/bin/sh\ncase \"$1\" in\n  Username*) printf '%s' \"$AIWX_GIT_USER\" ;;\n  *) printf '%s' \"$AIWX_GIT_PASS\" ;;\nesac\n"
		if err := os.WriteFile(askpass, []byte(script), 0o700); err != nil {
			cleanup()
			return nil, func() {}, err
		}
		env = append(env,
			"GIT_ASKPASS="+askpass,
			"AIWX_GIT_USER="+user,
			"AIWX_GIT_PASS="+pass,
			"GIT_TERMINAL_PROMPT=0",
		)

	case SCMAuthGitHubApp:
		// Token already minted into Password/Token at resolve time.
		user := a.Username
		if user == "" {
			user = "x-access-token"
		}
		askpass := filepath.Join(tmpDir, "askpass.sh")
		script := "#!/bin/sh\ncase \"$1\" in\n  Username*) printf '%s' \"$AIWX_GIT_USER\" ;;\n  *) printf '%s' \"$AIWX_GIT_PASS\" ;;\nesac\n"
		if err := os.WriteFile(askpass, []byte(script), 0o700); err != nil {
			cleanup()
			return nil, func() {}, err
		}
		env = append(env,
			"GIT_ASKPASS="+askpass,
			"AIWX_GIT_USER="+user,
			"AIWX_GIT_PASS="+a.Token,
			"GIT_TERMINAL_PROMPT=0",
		)
	}

	return env, cleanup, nil
}

// ─── GitHub App installation tokens ───────────────────────────────────────────

// MintGitHubInstallationToken creates a short-lived installation access token
// using a GitHub App JWT (RS256).
func MintGitHubInstallationToken(ctx context.Context, client *http.Client, auth *SCMAuth) (string, error) {
	jwt, err := signGitHubAppJWT(auth.GitHubAppID, auth.GitHubPrivateKeyPEM, time.Now())
	if err != nil {
		return "", err
	}

	base := strings.TrimRight(auth.GitHubBaseURL, "/")
	u := fmt.Sprintf("%s/app/installations/%s/access_tokens", base, url.PathEscape(auth.GitHubInstallationID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("github app token exchange: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var payload struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("decode installation token: %w", err)
	}
	if payload.Token == "" {
		return "", fmt.Errorf("github app token exchange: empty token")
	}
	return payload.Token, nil
}

// signGitHubAppJWT builds a GitHub App JWT (RS256, iss=app_id, exp=+9m).
func signGitHubAppJWT(appID, privateKeyPEM string, now time.Time) (string, error) {
	key, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	iat := now.Add(-60 * time.Second).Unix()
	exp := now.Add(9 * time.Minute).Unix()
	claims := fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":"%s"}`, iat, exp, appID)
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	signingInput := header + "." + payload

	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return key, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ─── Authenticated git runner ─────────────────────────────────────────────────

// runGitAuth runs git with SCM auth environment applied.
func (w *Worker) runGitAuth(ctx context.Context, auth *SCMAuth, dir string, args ...string) error {
	env, cleanup, err := auth.gitEnv(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, redactGitOutput(string(out)))
	}
	return nil
}

// redactGitOutput strips userinfo passwords from URLs in git stderr.
func redactGitOutput(s string) string {
	out := s
	for _, scheme := range []string{"https://", "http://", "ssh://", "git://", "ftps://", "ftp://"} {
		out = redactScheme(out, scheme)
	}
	return out
}

func redactScheme(s, scheme string) string {
	var b strings.Builder
	rest := s
	for {
		idx := strings.Index(rest, scheme)
		if idx < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:idx+len(scheme)])
		after := rest[idx+len(scheme):]
		at := strings.Index(after, "@")
		slash := strings.Index(after, "/")
		if at < 0 || (slash >= 0 && at > slash) {
			b.WriteString(after)
			return b.String()
		}
		userinfo := after[:at]
		if colon := strings.Index(userinfo, ":"); colon >= 0 {
			b.WriteString(userinfo[:colon])
			b.WriteString(":***")
		} else {
			b.WriteString(userinfo)
		}
		b.WriteString(after[at:])
		return b.String()
	}
}
