package inject

import (
	"testing"

	"github.com/google/uuid"
)

func TestBuildInjection_GitSSH(t *testing.T) {
	injector := &Injector{}

	cred := Credential{
		ID:   uuid.New(),
		Name: "git-ssh",
		Type: CredentialTypeGitSSH,
		Inputs: map[string]string{
			"ssh_key": "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----",
		},
	}

	injection, err := injector.BuildInjection([]Credential{cred}, uuid.New())
	if err != nil {
		t.Fatalf("BuildInjection failed: %v", err)
	}

	if len(injection.SecretData) == 0 {
		t.Error("SecretData should not be empty")
	}
	if _, ok := injection.SecretData["ssh_key"]; !ok {
		t.Error("ssh_key should be in SecretData")
	}
	if injection.EnvVars["GIT_SSH_COMMAND"] == "" {
		t.Error("GIT_SSH_COMMAND should be set")
	}
}

func TestBuildInjection_CloudAuth(t *testing.T) {
	injector := &Injector{}

	cred := Credential{
		ID:   uuid.New(),
		Name: "aws-creds",
		Type: CredentialTypeCloudAuth,
		Inputs: map[string]string{
			"access_key": "AKIAIOSFODNN7EXAMPLE",
			"secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			"region":     "us-east-1",
		},
	}

	injection, err := injector.BuildInjection([]Credential{cred}, uuid.New())
	if err != nil {
		t.Fatalf("BuildInjection failed: %v", err)
	}

	if _, ok := injection.SecretData["access_key"]; !ok {
		t.Error("access_key should be in SecretData")
	}
	if injection.EnvVars["AWS_REGION"] != "us-east-1" {
		t.Errorf("AWS_REGION = %q, want us-east-1", injection.EnvVars["AWS_REGION"])
	}
}

func TestRedactor_Redact(t *testing.T) {
	redactor := NewRedactor()
	redactor.AddSecret("my-secret-password")

	text := "The password is my-secret-password and it's secret"
	redacted := redactor.Redact(text)

	if redacted != "The password is *** and it's secret" {
		t.Errorf("Redact = %q", redacted)
	}
}

func TestRedactor_MultipleSecrets(t *testing.T) {
	redactor := NewRedactor()
	redactor.AddSecrets([]string{"secret1", "secret2", "secret3"})

	text := "Values: secret1, secret2, secret3"
	redacted := redactor.Redact(text)

	if redacted != "Values: ***, ***, ***" {
		t.Errorf("Redact = %q", redacted)
	}
}
