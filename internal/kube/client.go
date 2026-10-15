// Package kube provides Kubernetes integration for job execution.
package kube

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// JobSpec contains the specification for launching a job.
type JobSpec struct {
	JobID              uuid.UUID
	OrganizationID     uuid.UUID
	Image              string
	Playbook           string
	ProjectArtifactRef string
	CredentialSecrets  map[string]string
	InventoryContent   string
	ExtraVars          map[string]any
	Resources          ResourceRequirements
}

// ResourceRequirements defines pod resource limits.
type ResourceRequirements struct {
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

// JobStatus represents the status of a Kubernetes Job.
type JobStatus struct {
	Active    int
	Succeeded int
	Failed    int
	Ready     bool
}

// ─── Client ───────────────────────────────────────────────────────────────────

// Client wraps the Kubernetes clientset.
type Client struct {
	clientset kubernetes.Interface
	namespace string
	logger    *slog.Logger
}

// Config for Kubernetes client.
type Config struct {
	Kubeconfig string // Path to kubeconfig (empty for in-cluster)
	Namespace  string
}

// NewClient creates a new Kubernetes client.
func NewClient(cfg Config, logger *slog.Logger) (*Client, error) {
	var config *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("create k8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	ns := cfg.Namespace
	if ns == "" {
		ns = "default"
	}

	return &Client{
		clientset: clientset,
		namespace: ns,
		logger:    logger,
	}, nil
}

// ─── Job Operations ───────────────────────────────────────────────────────────

// CreateJob creates a Kubernetes Job for a job run.
func (c *Client) CreateJob(ctx context.Context, spec JobSpec) (string, error) {
	jobName := fmt.Sprintf("aiwx-job-%s", spec.JobID.String()[:8])

	// Build labels
	labels := map[string]string{
		"app":                    "aiwx",
		"aiwx.io/job-id":         spec.JobID.String(),
		"aiwx.io/organization":   spec.OrganizationID.String(),
	}

	// Build pod spec
	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{
			{
				Name:    "ansible",
				Image:   spec.Image,
				Command: []string{"ansible-playbook"},
				Args: []string{
					"-i", "/inventory/hosts.yml",
					"/project/" + spec.Playbook,
				},
				Env: buildEnvVars(spec.ExtraVars),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resourceQuantity(spec.Resources.CPURequest, "100m"),
						corev1.ResourceMemory: resourceQuantity(spec.Resources.MemoryRequest, "128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resourceQuantity(spec.Resources.CPULimit, "1"),
						corev1.ResourceMemory: resourceQuantity(spec.Resources.MemoryLimit, "512Mi"),
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "project", MountPath: "/project", ReadOnly: true},
					{Name: "inventory", MountPath: "/inventory", ReadOnly: true},
					{Name: "credentials", MountPath: "/credentials", ReadOnly: true},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "project",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "inventory",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: jobName + "-inventory",
						},
					},
				},
			},
			{
				Name: "credentials",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: jobName + "-credentials",
					},
				},
			},
		},
	}

	// Add init container for project artifact
	podSpec.InitContainers = []corev1.Container{
		{
			Name:    "fetch-artifact",
			Image:   "alpine:latest",
			Command: []string{"sh", "-c", fmt.Sprintf("cp /artifact/* /project/ && chmod -R 755 /project")},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "project", MountPath: "/project"},
				{Name: "artifact", MountPath: "/artifact", ReadOnly: true},
			},
		},
	}
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "artifact",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	// Create Job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: c.namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
			BackoffLimit:          int32Ptr(0),
			TTLSecondsAfterFinished: int32Ptr(300),
		},
	}

	_, err := c.clientset.BatchV1().Jobs(c.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create k8s job: %w", err)
	}

	c.logger.Info("kubernetes job created",
		"job_name", jobName,
		"namespace", c.namespace,
		"image", spec.Image,
	)

	return jobName, nil
}

// GetJobStatus returns the status of a Kubernetes Job.
func (c *Client) GetJobStatus(ctx context.Context, jobName string) (*JobStatus, error) {
	job, err := c.clientset.BatchV1().Jobs(c.namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get k8s job: %w", err)
	}

	status := &JobStatus{
		Active:    int(job.Status.Active),
		Succeeded: int(job.Status.Succeeded),
		Failed:    int(job.Status.Failed),
	}

	// Check if job is complete
	if job.Status.CompletionTime != nil {
		status.Ready = true
	}

	return status, nil
}

// DeleteJob deletes a Kubernetes Job.
func (c *Client) DeleteJob(ctx context.Context, jobName string) error {
	policy := metav1.DeletePropagationForeground
	err := c.clientset.BatchV1().Jobs(c.namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
	return err
}

// ─── Secret Operations ────────────────────────────────────────────────────────

// CreateSecret creates a Kubernetes Secret for job credentials.
func (c *Client) CreateSecret(ctx context.Context, jobName string, data map[string][]byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName + "-credentials",
			Namespace: c.namespace,
			Labels: map[string]string{
				"app":              "aiwx",
				"aiwx.io/job-name": jobName,
			},
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}

	_, err := c.clientset.CoreV1().Secrets(c.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create secret: %w", err)
	}

	return nil
}

// DeleteSecret deletes a Kubernetes Secret.
func (c *Client) DeleteSecret(ctx context.Context, jobName string) error {
	err := c.clientset.CoreV1().Secrets(c.namespace).Delete(ctx, jobName+"-credentials", metav1.DeleteOptions{})
	return err
}

// CreateConfigMap creates a Kubernetes ConfigMap for inventory.
func (c *Client) CreateConfigMap(ctx context.Context, jobName string, data map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName + "-inventory",
			Namespace: c.namespace,
		},
		Data: data,
	}

	_, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create configmap: %w", err)
	}

	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func buildEnvVars(extraVars map[string]any) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	for k, v := range extraVars {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: fmt.Sprintf("%v", v),
		})
	}
	return envVars
}

func resourceQuantity(value, defaultValue string) resource.Quantity {
	if value == "" {
		value = defaultValue
	}
	q, _ := resource.ParseQuantity(value)
	return q
}

func int32Ptr(i int32) *int32 {
	return &i
}
