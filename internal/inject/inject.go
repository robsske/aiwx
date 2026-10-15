// Package inject handles credential injection into execution pods.
package inject

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// CredentialType defines the type of credential.
type CredentialType string

const (
	CredentialTypeGitSSH     CredentialType = "git_ssh_key"
	CredentialTypeUserPass   CredentialType = "username_password"
	CredentialTypeVaultToken CredentialType = "vault_token"
	CredentialTypeK8sSecret  CredentialType = "k8s_secret_reference"
	CredentialTypeCloudAuth  CredentialType = "cloud_auth"
)

// Credential represents a credential to inject.
type Credential struct {
	ID               uuid.UUID
	Name             string
	Type             CredentialType
	Inputs           map[string]string
	OrganizationID   uuid.UUID
}

// Injection contains the data to inject into a pod.
type Injection struct {
	SecretData   map[string][]byte
	ConfigData   map[string]string
	EnvVars      map[string]string
	VolumeMounts []VolumeMount
}

// VolumeMount describes a volume mount for credentials.
type VolumeMount struct {
	Name      string
	MountPath string
	FileName  string
	ReadOnly  bool
}

// ─── Injector ─────────────────────────────────────────────────────────────────

// Injector handles credential injection.
type Injector struct {
	logger *slog.Logger
}

// NewInjector creates a new credential injector.
func NewInjector(logger *slog.Logger) *Injector {
	return &Injector{logger: logger}
}

// BuildInjection creates injection data from a set of credentials.
func (i *Injector) BuildInjection(creds []Credential, jobID uuid.UUID) (*Injection, error) {
	injection := &Injection{
		SecretData:   make(map[string][]byte),
		ConfigData:   make(map[string]string),
		EnvVars:      make(map[string]string),
		VolumeMounts: make([]VolumeMount, 0),
	}

	for _, cred := range creds {
		if err := i.injectCredential(injection, cred, jobID); err != nil {
			return nil, fmt.Errorf("inject credential %s: %w", cred.Name, err)
		}
	}

	return injection, nil
}

// injectCredential injects a single credential.
func (i *Injector) injectCredential(injection *Injection, cred Credential, jobID uuid.UUID) error {
	switch cred.Type {
	case CredentialTypeGitSSH:
		return i.injectGitSSH(injection, cred)
	case CredentialTypeUserPass:
		return i.injectUserPass(injection, cred)
	case CredentialTypeVaultToken:
		return i.injectVaultToken(injection, cred)
	case CredentialTypeCloudAuth:
		return i.injectCloudAuth(injection, cred)
	default:
		return fmt.Errorf("unsupported credential type: %s", cred.Type)
	}
}

// injectGitSSH injects a Git SSH key.
func (i *Injector) injectGitSSH(injection *Injection, cred Credential) error {
	sshKey, ok := cred.Inputs["ssh_key"]
	if !ok {
		return fmt.Errorf("missing ssh_key input")
	}

	// Store as secret data
	injection.SecretData["ssh_key"] = []byte(sshKey)

	// Mount as file
	injection.VolumeMounts = append(injection.VolumeMounts, VolumeMount{
		Name:      "credentials",
		MountPath: "/credentials",
		FileName:  "ssh_key",
		ReadOnly:  true,
	})

	// Set GIT_SSH_COMMAND to use the key
	injection.EnvVars["GIT_SSH_COMMAND"] = "ssh -i /credentials/ssh_key -o StrictHostKeyChecking=no"

	return nil
}

// injectUserPass injects username/password credentials.
func (i *Injector) injectUserPass(injection *Injection, cred Credential) error {
	username, ok := cred.Inputs["username"]
	if !ok {
		return fmt.Errorf("missing username input")
	}
	password, ok := cred.Inputs["password"]
	if !ok {
		return fmt.Errorf("missing password input")
	}

	// Store as secret data
	injection.SecretData["username"] = []byte(username)
	injection.SecretData["password"] = []byte(password)

	// Set environment variables (not recommended for secrets, but some tools need it)
	// Use file-based injection instead
	injection.VolumeMounts = append(injection.VolumeMounts, VolumeMount{
		Name:      "credentials",
		MountPath: "/credentials",
		FileName:  "username",
		ReadOnly:  true,
	})

	return nil
}

// injectVaultToken injects a Vault token.
func (i *Injector) injectVaultToken(injection *Injection, cred Credential) error {
	vaultURL, ok := cred.Inputs["vault_url"]
	if !ok {
		return fmt.Errorf("missing vault_url input")
	}
	vaultToken, ok := cred.Inputs["vault_token"]
	if !ok {
		return fmt.Errorf("missing vault_token input")
	}

	// Store as secret data
	injection.SecretData["vault_token"] = []byte(vaultToken)

	// Set environment variables for Vault
	injection.EnvVars["VAULT_ADDR"] = vaultURL
	injection.EnvVars["VAULT_TOKEN_FILE"] = "/credentials/vault_token"

	injection.VolumeMounts = append(injection.VolumeMounts, VolumeMount{
		Name:      "credentials",
		MountPath: "/credentials",
		FileName:  "vault_token",
		ReadOnly:  true,
	})

	return nil
}

// injectCloudAuth injects cloud authentication credentials.
func (i *Injector) injectCloudAuth(injection *Injection, cred Credential) error {
	accessKey, ok := cred.Inputs["access_key"]
	if !ok {
		return fmt.Errorf("missing access_key input")
	}
	secretKey, ok := cred.Inputs["secret_key"]
	if !ok {
		return fmt.Errorf("missing secret_key input")
	}

	// Store as secret data
	injection.SecretData["access_key"] = []byte(accessKey)
	injection.SecretData["secret_key"] = []byte(secretKey)

	// Set AWS environment variables by default
	injection.EnvVars["AWS_ACCESS_KEY_ID_FILE"] = "/credentials/access_key"
	injection.EnvVars["AWS_SECRET_ACCESS_KEY_FILE"] = "/credentials/secret_key"

	if region, ok := cred.Inputs["region"]; ok {
		injection.EnvVars["AWS_REGION"] = region
	}

	injection.VolumeMounts = append(injection.VolumeMounts, VolumeMount{
		Name:      "credentials",
		MountPath: "/credentials",
		FileName:  "access_key",
		ReadOnly:  true,
	})

	return nil
}

// ─── Redaction ────────────────────────────────────────────────────────────────

// Redactor redacts secret values from log output.
type Redactor struct {
	secrets []string
}

// NewRedactor creates a new redactor.
func NewRedactor() *Redactor {
	return &Redactor{}
}

// AddSecret adds a secret value to redact.
func (r *Redactor) AddSecret(secret string) {
	if secret != "" {
		r.secrets = append(r.secrets, secret)
	}
}

// AddSecrets adds multiple secret values to redact.
func (r *Redactor) AddSecrets(secrets []string) {
	for _, s := range secrets {
		r.AddSecret(s)
	}
}

// Redact replaces secret values in text with "***".
func (r *Redactor) Redact(text string) string {
	result := text
	for _, secret := range r.secrets {
		result = replaceAll(result, secret, "***")
	}
	return result
}

// RedactBytes replaces secret values in byte slice with "***".
func (r *Redactor) RedactBytes(data []byte) []byte {
	return []byte(r.Redact(string(data)))
}

func replaceAll(s, old, replacement string) string {
	if old == "" {
		return s
	}
	return strings.ReplaceAll(s, old, replacement)
}
