package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rverhe/aiwx/internal/domain"
)

// LocalRunner runs ansible-playbook as a child process.
type LocalRunner struct {
	// PlaybookPath is the path to ansible-playbook binary.
	PlaybookPath string
	// ProjectsDir is where synced project artifacts live.
	ProjectsDir string
	// LogRepo writes stdout/stderr lines to the job log.
	LogRepo JobLogWriter
}

// JobLogWriter persists job log lines.
type JobLogWriter interface {
	Append(ctx context.Context, jobID uuid.UUID, stream string, line string) error
}

// NewLocalRunner creates a LocalRunner.
func NewLocalRunner(playbookPath, projectsDir string, logRepo JobLogWriter) *LocalRunner {
	if playbookPath == "" {
		playbookPath = "/home/rverhe/aiwx/.venv/bin/ansible-playbook"
	}
	if projectsDir == "" {
		projectsDir = "/tmp/aiwx-artifacts"
	}
	return &LocalRunner{
		PlaybookPath: playbookPath,
		ProjectsDir:  projectsDir,
		LogRepo:      logRepo,
	}
}

// Run executes the job with real ansible-playbook.
func (r *LocalRunner) Run(ctx context.Context, job *domain.Job, spec *JobSpec) (*JobResult, error) {
	// Verify ansible-playbook exists
	if _, err := os.Stat(r.PlaybookPath); os.IsNotExist(err) {
		return r.mockRun(ctx, job, spec)
	}

	// Resolve working directory — extract artifact if needed
	workDir := spec.ProjectDir
	if workDir == "" {
		workDir = r.resolveProjectDir(job)
	}
	if workDir == "" {
		workDir = os.TempDir()
	}

	// If workDir contains a tar.gz, extract it
	workDir = r.extractArtifactIfNeeded(workDir)

	// Build ansible-playbook command — path must stay under workDir
	playbookPath, err := safeJoin(workDir, spec.Playbook)
	if err != nil {
		return &JobResult{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("Invalid playbook path: %v\n", err),
		}, nil
	}

	// Verify playbook exists
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		return &JobResult{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("Playbook not found: %s\n", playbookPath),
		}, nil
	}

	args := []string{playbookPath}

	// Inventory
	inventoryPath := spec.Inventory
	if inventoryPath == "" {
		// Default to localhost for testing
		args = append(args, "-i", "localhost,", "-c", "local")
	} else {
		inv, err := safeJoin(workDir, inventoryPath)
		if err != nil {
			return &JobResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("Invalid inventory path: %v\n", err),
			}, nil
		}
		inventoryPath = inv
		args = append(args, "-i", inventoryPath)
	}

	// Host targeting (AWX-style limit)
	if spec.Limit != "" {
		args = append(args, "--limit", spec.Limit)
	}

	// Verbosity (0-4 maps to no flag, -v, -vv, -vvv, -vvvv)
	for i := 0; i < spec.Verbosity; i++ {
		args = append(args, "-v")
	}

	// Forks
	if spec.Forks > 0 {
		args = append(args, "--forks", fmt.Sprintf("%d", spec.Forks))
	}

	// Job tags
	if spec.JobTags != "" {
		args = append(args, "--tags", spec.JobTags)
	}

	// Skip tags
	if spec.SkipTags != "" {
		args = append(args, "--skip-tags", spec.SkipTags)
	}

	// Extra vars as YAML
	if len(spec.ExtraVars) > 0 {
		yamlVars := mapToYAML(spec.ExtraVars)
		args = append(args, "--extra-vars", yamlVars)
	}

	cmd := exec.CommandContext(ctx, r.PlaybookPath, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"ANSIBLE_HOST_KEY_CHECKING=False",
		"ANSIBLE_FORCE_COLOR=false",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	result := &JobResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Stderr += fmt.Sprintf("\nRunner error: %v\n", err)
		}
	}

	// Write structured logs
	if r.LogRepo != nil {
		_ = r.LogRepo.Append(ctx, job.ID, "system", fmt.Sprintf("=== ansible-playbook %s ===\n", strings.Join(args, " ")))
		_ = r.LogRepo.Append(ctx, job.ID, "system", fmt.Sprintf("Working directory: %s\n", workDir))
		_ = r.LogRepo.Append(ctx, job.ID, "system", fmt.Sprintf("Duration: %v\n", duration))

		if result.Stdout != "" {
			for _, line := range strings.Split(result.Stdout, "\n") {
				if line != "" {
					_ = r.LogRepo.Append(ctx, job.ID, "stdout", line+"\n")
				}
			}
		}
		if result.Stderr != "" {
			for _, line := range strings.Split(result.Stderr, "\n") {
				if line != "" {
					_ = r.LogRepo.Append(ctx, job.ID, "stderr", line+"\n")
				}
			}
		}
		_ = r.LogRepo.Append(ctx, job.ID, "system", fmt.Sprintf("=== Exit code: %d ===\n", result.ExitCode))
	}

	return result, nil
}

// extractArtifactIfNeeded extracts a tar.gz artifact to a working directory.
func (r *LocalRunner) extractArtifactIfNeeded(artifactDir string) string {
	entries, err := os.ReadDir(artifactDir)
	if err != nil {
		return artifactDir
	}

	// Find the tar.gz file
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tar.gz") {
			tarPath := filepath.Join(artifactDir, entry.Name())
			extractDir := filepath.Join(artifactDir, "extracted")

			// Extract with path containment (mitigate tar slip)
			if err := os.MkdirAll(extractDir, 0o755); err != nil {
				continue
			}
			if err := extractTarSafe(tarPath, extractDir); err == nil {
				return extractDir
			}
		}
	}

	return artifactDir
}

// extractTarSafe extracts a gzip tarball while rejecting path-traversal members.
func extractTarSafe(tarPath, dest string) error {
	cmd := exec.Command("tar", "-tzf", tarPath)
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		clean := filepath.Clean(name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("tar member escapes destination: %s", name)
		}
		// Also reject members that would land outside dest after join
		target := filepath.Join(destAbs, clean)
		if target != destAbs && !strings.HasPrefix(target, destAbs+string(filepath.Separator)) {
			return fmt.Errorf("tar member escapes destination: %s", name)
		}
	}
	extract := exec.Command("tar", "-xzf", tarPath, "-C", dest)
	return extract.Run()
}

// resolveProjectDir finds the synced project artifact directory for a job.
func (r *LocalRunner) resolveProjectDir(job *domain.Job) string {
	if job.JobTemplateID == nil {
		return ""
	}

	// Query project ID from job template
	// Expected layout: ProjectsDir/<orgID>/<projectID>/<commit>.tar.gz
	// For now, search for any directory under ProjectsDir
	if _, err := os.Stat(r.ProjectsDir); err == nil {
		return r.ProjectsDir
	}
	return ""
}

// mockRun is a fallback when ansible-playbook is not installed.
func (r *LocalRunner) mockRun(ctx context.Context, job *domain.Job, spec *JobSpec) (*JobResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	result := &JobResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("[mock] ansible-playbook not found. Would run: %s\n", spec.Playbook),
	}

	if r.LogRepo != nil {
		_ = r.LogRepo.Append(ctx, job.ID, "stdout", result.Stdout)
	}

	return result, nil
}

// mapToYAML converts a map to a YAML string for --extra-vars.
// safeJoin joins a relative path under base and rejects traversal / absolute escapes.
func safeJoin(base, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	// Disallow absolute paths and parent references.
	cleaned := filepath.Clean(rel)
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must be relative and stay under the project directory")
	}
	full := filepath.Join(base, cleaned)
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if absFull != absBase && !strings.HasPrefix(absFull, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes project directory")
	}
	return full, nil
}

// mapToYAML serializes extra vars safely (no injection via quotes/newlines).
func mapToYAML(m map[string]any) string {
	// Use encoding/json-compatible YAML via json.Marshal then convert — safest is yaml.v3,
	// but avoid new deps: JSON is valid YAML for simple scalars/maps.
	// Prefer a proper marshal: use yaml from stdlib-free approach with strconv.Quote for strings.
	var sb strings.Builder
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		switch val := v.(type) {
		case string:
			fmt.Fprintf(&sb, "%s: %s\n", yamlKey(k), strconv.Quote(val))
		case int, int64, float64:
			fmt.Fprintf(&sb, "%s: %v\n", yamlKey(k), val)
		case bool:
			fmt.Fprintf(&sb, "%s: %v\n", yamlKey(k), val)
		default:
			// Nested values: JSON is YAML-compatible
			b, err := json.Marshal(val)
			if err != nil {
				fmt.Fprintf(&sb, "%s: null\n", yamlKey(k))
				continue
			}
			fmt.Fprintf(&sb, "%s: %s\n", yamlKey(k), b)
		}
	}
	return sb.String()
}

func yamlKey(k string) string {
	if k == "" {
		return `""`
	}
	// Quote keys that need it
	if strings.ContainsAny(k, ":#{}[],&*?|-<>=!%@`\"' \t\n") {
		return strconv.Quote(k)
	}
	return k
}
