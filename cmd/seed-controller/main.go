package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"task-api-huma-mongo/internal/config"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	group   = "tasks.huma.io"
	version = "v1alpha1"
	kind    = "TaskSeed"
)

const (
	phasePending   = "Pending"
	phaseRunning   = "Running"
	phaseSucceeded = "Succeeded"
	phaseFailed    = "Failed"
)

type ControllerConfig struct {
	Namespace                string
	PollInterval             time.Duration
	DefaultMongoURI          string
	DefaultMongoDB           string
	DefaultMongoCollection   string
	DefaultSeedCount         int
	DefaultRandomSeed        int64
	DefaultSeedMode          string
	DefaultSeedTitlePrefix   string
	DefaultSeedTags          []string
	DefaultSeedDoneRatio     float64
	DefaultSeedTagCountMin   int
	DefaultSeedTagCountMax   int
	DefaultJobTTLSeconds     int32
	DefaultJobBackoffLimit   int32
	DefaultJobActiveDeadline int64
	SeedJobImage             string
	SeedJobPullPolicy        corev1.PullPolicy
}

type TaskSeed struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TaskSeedSpec   `json:"spec,omitempty"`
	Status            TaskSeedStatus `json:"status,omitempty"`
}

type TaskSeedSpec struct {
	Size           *int             `json:"size,omitempty"`
	Seed           *int64           `json:"seed,omitempty"`
	Mode           string           `json:"mode,omitempty"`
	SeedVersion    string           `json:"seedVersion,omitempty"`
	TitlePrefix    string           `json:"titlePrefix,omitempty"`
	Tags           []string         `json:"tags,omitempty"`
	DoneRatio      *float64         `json:"doneRatio,omitempty"`
	TagCountMin    *int             `json:"tagCountMin,omitempty"`
	TagCountMax    *int             `json:"tagCountMax,omitempty"`
	CreatedAtStart string           `json:"createdAtStart,omitempty"`
	CreatedAtEnd   string           `json:"createdAtEnd,omitempty"`
	Database       string           `json:"database,omitempty"`
	Collection     string           `json:"collection,omitempty"`
	MongoDB        TaskSeedMongoDB  `json:"mongodb,omitempty"`
	Job            TaskSeedJobSpec  `json:"job,omitempty"`
	MaintenanceSchedule string      `json:"maintenanceSchedule,omitempty"`
}

type TaskSeedMongoDB struct {
	URI          string     `json:"uri,omitempty"`
	URISecretRef *SecretRef `json:"uriSecretRef,omitempty"`
}

type SecretRef struct {
	Name string `json:"name,omitempty"`
	Key  string `json:"key,omitempty"`
}

type TaskSeedJobSpec struct {
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
	BackoffLimit            *int32 `json:"backoffLimit,omitempty"`
	ActiveDeadlineSeconds   *int64 `json:"activeDeadlineSeconds,omitempty"`
}

type TaskSeedStatus struct {
	Phase              string      `json:"phase,omitempty"`
	Message            string      `json:"message,omitempty"`
	JobName            string      `json:"jobName,omitempty"`
	AppliedHash        string      `json:"appliedHash,omitempty"`
	LastRunTime        *metav1.Time `json:"lastRunTime,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
}

type SeedInput struct {
	Size           int       `json:"size"`
	RandomSeed     int64     `json:"seed"`
	Mode           string    `json:"mode"`
	SeedVersion    string    `json:"seedVersion,omitempty"`
	TitlePrefix    string    `json:"titlePrefix,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	DoneRatio      float64   `json:"doneRatio"`
	TagCountMin    int       `json:"tagCountMin"`
	TagCountMax    int       `json:"tagCountMax"`
	CreatedAtStart string    `json:"createdAtStart,omitempty"`
	CreatedAtEnd   string    `json:"createdAtEnd,omitempty"`
	Database       string    `json:"database"`
	Collection     string    `json:"collection"`
	MongoURI       string    `json:"mongoUri,omitempty"`
	MongoURISecret *SecretRef `json:"mongoUriSecretRef,omitempty"`
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := loadConfig()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	restCfg, err := buildKubeConfig()
	if err != nil {
		slog.Error("kubeconfig error", "err", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		slog.Error("kubernetes client error", "err", err)
		os.Exit(1)
	}
	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		slog.Error("dynamic client error", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		cancel()
	}()

	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: "taskseeds"}
	resource := dynamicClient.Resource(gvr).Namespace(cfg.Namespace)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		if err := reconcileAll(ctx, cfg, clientset, resource); err != nil {
			slog.Error("reconcile error", "err", err)
		}

		select {
		case <-ctx.Done():
			slog.Info("seed controller stopped")
			return
		case <-ticker.C:
		}
	}
}

func loadConfig() (ControllerConfig, error) {
	pollInterval, err := config.DurationEnv("SEED_CONTROLLER_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return ControllerConfig{}, fmt.Errorf("invalid SEED_CONTROLLER_POLL_INTERVAL: %w", err)
	}
	defaultSeedCount, err := config.IntEnv("DEFAULT_SEED_COUNT", 50)
	if err != nil {
		return ControllerConfig{}, fmt.Errorf("invalid DEFAULT_SEED_COUNT: %w", err)
	}
	defaultRandomSeed, err := config.Int64Env("DEFAULT_SEED_RANDOM_SEED", 1)
	if err != nil {
		return ControllerConfig{}, fmt.Errorf("invalid DEFAULT_SEED_RANDOM_SEED: %w", err)
	}
	defaultDoneRatio, err := config.FloatEnv("DEFAULT_SEED_DONE_RATIO", 0.3)
	if err != nil {
		return ControllerConfig{}, fmt.Errorf("invalid DEFAULT_SEED_DONE_RATIO: %w", err)
	}
	tagMin, err := config.IntEnv("DEFAULT_SEED_TAG_COUNT_MIN", 0)
	if err != nil {
		return ControllerConfig{}, fmt.Errorf("invalid DEFAULT_SEED_TAG_COUNT_MIN: %w", err)
	}
	tagMax, err := config.IntEnv("DEFAULT_SEED_TAG_COUNT_MAX", 2)
	if err != nil {
		return ControllerConfig{}, fmt.Errorf("invalid DEFAULT_SEED_TAG_COUNT_MAX: %w", err)
	}
	jobTTL, err := config.IntEnv("DEFAULT_JOB_TTL_SECONDS_AFTER_FINISHED", 300)
	if err != nil {
		return ControllerConfig{}, fmt.Errorf("invalid DEFAULT_JOB_TTL_SECONDS_AFTER_FINISHED: %w", err)
	}
	jobBackoff, err := config.IntEnv("DEFAULT_JOB_BACKOFF_LIMIT", 1)
	if err != nil {
		return ControllerConfig{}, fmt.Errorf("invalid DEFAULT_JOB_BACKOFF_LIMIT: %w", err)
	}
	jobDeadline, err := config.Int64Env("DEFAULT_JOB_ACTIVE_DEADLINE_SECONDS", 300)
	if err != nil {
		return ControllerConfig{}, fmt.Errorf("invalid DEFAULT_JOB_ACTIVE_DEADLINE_SECONDS: %w", err)
	}

	namespace := config.GetEnv("SEED_CONTROLLER_NAMESPACE", "")
	if namespace == "" {
		namespace = config.GetEnv("POD_NAMESPACE", "default")
	}

	pullPolicy := corev1.PullPolicy(config.GetEnv("SEED_JOB_IMAGE_PULL_POLICY", "IfNotPresent"))
	seedJobImage := config.GetEnv("SEED_JOB_IMAGE", "")
	if seedJobImage == "" {
		return ControllerConfig{}, errors.New("SEED_JOB_IMAGE is required")
	}

	return ControllerConfig{
		Namespace:                namespace,
		PollInterval:             pollInterval,
		DefaultMongoURI:          config.GetEnv("DEFAULT_MONGODB_URI", ""),
		DefaultMongoDB:           config.GetEnv("DEFAULT_MONGODB_DB", "taskdb"),
		DefaultMongoCollection:   config.GetEnv("DEFAULT_MONGODB_COLLECTION", "tasks"),
		DefaultSeedCount:         defaultSeedCount,
		DefaultRandomSeed:        defaultRandomSeed,
		DefaultSeedMode:          config.GetEnv("DEFAULT_SEED_MODE", "upsert"),
		DefaultSeedTitlePrefix:   config.GetEnv("DEFAULT_SEED_TITLE_PREFIX", "Task"),
		DefaultSeedTags:          config.SplitCommaList(config.GetEnv("DEFAULT_SEED_TAGS", "demo,seed")),
		DefaultSeedDoneRatio:     defaultDoneRatio,
		DefaultSeedTagCountMin:   tagMin,
		DefaultSeedTagCountMax:   tagMax,
		DefaultJobTTLSeconds:     int32(jobTTL),
		DefaultJobBackoffLimit:   int32(jobBackoff),
		DefaultJobActiveDeadline: jobDeadline,
		SeedJobImage:             seedJobImage,
		SeedJobPullPolicy:        pullPolicy,
	}, nil
}

func buildKubeConfig() (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	kubeconfig := config.GetEnv("KUBECONFIG", "")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			kubeconfig = fmt.Sprintf("%s\\.kube\\config", home)
		}
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func reconcileAll(ctx context.Context, cfg ControllerConfig, clientset *kubernetes.Clientset, resource dynamic.ResourceInterface) error {
	list, err := resource.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for i := range list.Items {
		var seed TaskSeed
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[i].Object, &seed); err != nil {
			slog.Error("decode taskseed error", "err", err)
			continue
		}
		if err := reconcileSeed(ctx, cfg, clientset, resource, &seed); err != nil {
			slog.Error("reconcile taskseed failed", "name", seed.Name, "err", err)
		}
	}
	return nil
}

func reconcileSeed(ctx context.Context, cfg ControllerConfig, clientset *kubernetes.Clientset, resource dynamic.ResourceInterface, seed *TaskSeed) error {
	input, err := resolveSeedInput(cfg, seed.Spec)
	if err != nil {
		return patchStatus(ctx, resource, seed, TaskSeedStatus{
			Phase:              phaseFailed,
			Message:            err.Error(),
			ObservedGeneration: seed.Generation,
		})
	}

	specHash, err := hashSpec(input)
	if err != nil {
		return err
	}
	jobName := buildJobName(seed.Name, specHash)

	job, err := clientset.BatchV1().Jobs(seed.Namespace).Get(ctx, jobName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if seed.Status.AppliedHash == specHash && seed.Status.Phase == phaseSucceeded {
			return nil
		}
		activeJob, err := findActiveJob(ctx, clientset, seed.Namespace, seed.Name)
		if err != nil {
			return err
		}
		if activeJob != "" {
			return patchStatus(ctx, resource, seed, TaskSeedStatus{
				Phase:              phaseRunning,
				Message:            "waiting for active seed job",
				JobName:            activeJob,
				AppliedHash:        seed.Status.AppliedHash,
				ObservedGeneration: seed.Generation,
			})
		}
		job = buildJob(cfg, seed, input, specHash, jobName)
		if _, err := clientset.BatchV1().Jobs(seed.Namespace).Create(ctx, job, metav1.CreateOptions{}); err != nil {
			return patchStatus(ctx, resource, seed, TaskSeedStatus{
				Phase:              phaseFailed,
				Message:            err.Error(),
				ObservedGeneration: seed.Generation,
			})
		}
		if err := ensureMaintenanceCronJob(ctx, cfg, clientset, seed, input); err != nil {
			slog.Error("maintenance reconcile error", "name", seed.Name, "err", err)
		}
		return patchStatus(ctx, resource, seed, TaskSeedStatus{
			Phase:              phasePending,
			JobName:            jobName,
			AppliedHash:        specHash,
			ObservedGeneration: seed.Generation,
		})
	}
	if err != nil {
		return err
	}

	if err := ensureMaintenanceCronJob(ctx, cfg, clientset, seed, input); err != nil {
		slog.Error("maintenance reconcile error", "name", seed.Name, "err", err)
	}

	phase, message, lastRun := deriveJobPhase(job)
	status := TaskSeedStatus{
		Phase:              phase,
		Message:            message,
		JobName:            jobName,
		AppliedHash:        specHash,
		LastRunTime:        lastRun,
		ObservedGeneration: seed.Generation,
	}
	return patchStatus(ctx, resource, seed, status)
}

func resolveSeedInput(cfg ControllerConfig, spec TaskSeedSpec) (SeedInput, error) {
	size := cfg.DefaultSeedCount
	if spec.Size != nil {
		size = *spec.Size
	}
	if size <= 0 {
		return SeedInput{}, errors.New("spec.size must be greater than zero")
	}

	randomSeed := cfg.DefaultRandomSeed
	if spec.Seed != nil {
		randomSeed = *spec.Seed
	}

	mode := strings.ToLower(spec.Mode)
	if mode == "" {
		mode = strings.ToLower(cfg.DefaultSeedMode)
	}
	switch mode {
	case "append", "replace", "upsert", "maintain":
	default:
		return SeedInput{}, fmt.Errorf("invalid spec.mode: %s", spec.Mode)
	}

	doneRatio := cfg.DefaultSeedDoneRatio
	if spec.DoneRatio != nil {
		doneRatio = *spec.DoneRatio
	}
	if doneRatio < 0 || doneRatio > 1 {
		return SeedInput{}, errors.New("spec.doneRatio must be between 0 and 1")
	}

	tagMin := cfg.DefaultSeedTagCountMin
	if spec.TagCountMin != nil {
		tagMin = *spec.TagCountMin
	}
	tagMax := cfg.DefaultSeedTagCountMax
	if spec.TagCountMax != nil {
		tagMax = *spec.TagCountMax
	}
	if tagMin < 0 || tagMax < 0 || tagMax < tagMin {
		return SeedInput{}, errors.New("invalid tag count range")
	}

	dbName := cfg.DefaultMongoDB
	if spec.Database != "" {
		dbName = spec.Database
	}
	collection := cfg.DefaultMongoCollection
	if spec.Collection != "" {
		collection = spec.Collection
	}
	mongoURI := cfg.DefaultMongoURI
	mongoSecret := spec.MongoDB.URISecretRef
	if spec.MongoDB.URI != "" {
		mongoURI = spec.MongoDB.URI
	}
	if mongoSecret != nil {
		mongoURI = ""
	}
	if mongoURI == "" && mongoSecret == nil {
		return SeedInput{}, errors.New("mongodb uri is required")
	}

	tags := spec.Tags
	if len(tags) == 0 {
		tags = cfg.DefaultSeedTags
	}

	titlePrefix := spec.TitlePrefix
	if titlePrefix == "" {
		titlePrefix = cfg.DefaultSeedTitlePrefix
	}

	return SeedInput{
		Size:           size,
		RandomSeed:     randomSeed,
		Mode:           mode,
		SeedVersion:    spec.SeedVersion,
		TitlePrefix:    titlePrefix,
		Tags:           tags,
		DoneRatio:      doneRatio,
		TagCountMin:    tagMin,
		TagCountMax:    tagMax,
		CreatedAtStart: spec.CreatedAtStart,
		CreatedAtEnd:   spec.CreatedAtEnd,
		Database:       dbName,
		Collection:     collection,
		MongoURI:       mongoURI,
		MongoURISecret: mongoSecret,
	}, nil
}

func buildJob(cfg ControllerConfig, seed *TaskSeed, input SeedInput, specHash, jobName string) *batchv1.Job {
	labels := buildBaseLabels(seed.Name)
	labels["taskseed.huma.io/hash"] = shortHash(specHash)

	env := buildEnv(input, input.Mode)

	jobTTL, backoff, deadline := resolveJobSettings(cfg, seed)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            jobName,
			Namespace:       seed.Namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{ownerRef(seed)},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: int32Ptr(jobTTL),
			BackoffLimit:            int32Ptr(backoff),
			ActiveDeadlineSeconds:   int64Ptr(deadline),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					AutomountServiceAccountToken: boolPtr(false),
					Containers: []corev1.Container{
						{
							Name:            "seeder",
							Image:           cfg.SeedJobImage,
							ImagePullPolicy: cfg.SeedJobPullPolicy,
							Command:         []string{"/app/seeder"},
							Env:             env,
						},
					},
				},
			},
		},
	}
}

func ensureMaintenanceCronJob(ctx context.Context, cfg ControllerConfig, clientset *kubernetes.Clientset, seed *TaskSeed, input SeedInput) error {
	schedule := strings.TrimSpace(seed.Spec.MaintenanceSchedule)
	name := buildMaintenanceCronJobName(seed.Name)
	if schedule == "" {
		err := clientset.BatchV1().CronJobs(seed.Namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	desired := buildMaintenanceCronJob(cfg, seed, input, schedule, name)
	existing, err := clientset.BatchV1().CronJobs(seed.Namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = clientset.BatchV1().CronJobs(seed.Namespace).Create(ctx, desired, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	desired.ResourceVersion = existing.ResourceVersion
	_, err = clientset.BatchV1().CronJobs(seed.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
	return err
}

func buildMaintenanceCronJob(cfg ControllerConfig, seed *TaskSeed, input SeedInput, schedule, name string) *batchv1.CronJob {
	labels := buildBaseLabels(seed.Name)
	labels["taskseed.huma.io/maintenance"] = "true"

	env := buildEnv(input, "maintain")
	jobTTL, backoff, deadline := resolveJobSettings(cfg, seed)

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       seed.Namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{ownerRef(seed)},
		},
		Spec: batchv1.CronJobSpec{
			Schedule:          schedule,
			ConcurrencyPolicy: batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: batchv1.JobSpec{
					TTLSecondsAfterFinished: int32Ptr(jobTTL),
					BackoffLimit:            int32Ptr(backoff),
					ActiveDeadlineSeconds:   int64Ptr(deadline),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							RestartPolicy:                 corev1.RestartPolicyNever,
							AutomountServiceAccountToken: boolPtr(false),
							Containers: []corev1.Container{
								{
									Name:            "seeder",
									Image:           cfg.SeedJobImage,
									ImagePullPolicy: cfg.SeedJobPullPolicy,
									Command:         []string{"/app/seeder"},
									Env:             env,
								},
							},
						},
					},
				},
			},
			SuccessfulJobsHistoryLimit: int32Ptr(1),
			FailedJobsHistoryLimit:     int32Ptr(1),
		},
	}
}

func buildMaintenanceCronJobName(seedName string) string {
	name := fmt.Sprintf("taskseed-%s-maintain", strings.ToLower(seedName))
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "-")
	}
	return name
}

func resolveJobSettings(cfg ControllerConfig, seed *TaskSeed) (int32, int32, int64) {
	jobTTL := cfg.DefaultJobTTLSeconds
	if seed.Spec.Job.TTLSecondsAfterFinished != nil {
		jobTTL = *seed.Spec.Job.TTLSecondsAfterFinished
	}
	backoff := cfg.DefaultJobBackoffLimit
	if seed.Spec.Job.BackoffLimit != nil {
		backoff = *seed.Spec.Job.BackoffLimit
	}
	deadline := cfg.DefaultJobActiveDeadline
	if seed.Spec.Job.ActiveDeadlineSeconds != nil {
		deadline = *seed.Spec.Job.ActiveDeadlineSeconds
	}
	return jobTTL, backoff, deadline
}

func buildBaseLabels(seedName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "task-seeder",
		"app.kubernetes.io/instance":   seedName,
		"app.kubernetes.io/managed-by": "task-seed-controller",
		"taskseed.huma.io/name":        seedName,
	}
}

func buildEnv(input SeedInput, mode string) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "MONGODB_DB", Value: input.Database},
		{Name: "MONGODB_COLLECTION", Value: input.Collection},
		{Name: "SEED_COUNT", Value: fmt.Sprintf("%d", input.Size)},
		{Name: "SEED_MODE", Value: mode},
		{Name: "SEED_RANDOM_SEED", Value: fmt.Sprintf("%d", input.RandomSeed)},
		{Name: "SEED_TITLE_PREFIX", Value: input.TitlePrefix},
		{Name: "SEED_DONE_RATIO", Value: fmt.Sprintf("%g", input.DoneRatio)},
		{Name: "SEED_TAG_COUNT_MIN", Value: fmt.Sprintf("%d", input.TagCountMin)},
		{Name: "SEED_TAG_COUNT_MAX", Value: fmt.Sprintf("%d", input.TagCountMax)},
	}
	if input.SeedVersion != "" {
		env = append(env, corev1.EnvVar{Name: "SEED_VERSION", Value: input.SeedVersion})
	}
	if len(input.Tags) > 0 {
		env = append(env, corev1.EnvVar{Name: "SEED_TAGS", Value: strings.Join(input.Tags, ",")})
	}
	if input.CreatedAtStart != "" {
		env = append(env, corev1.EnvVar{Name: "SEED_CREATED_AT_START", Value: input.CreatedAtStart})
	}
	if input.CreatedAtEnd != "" {
		env = append(env, corev1.EnvVar{Name: "SEED_CREATED_AT_END", Value: input.CreatedAtEnd})
	}
	if input.MongoURISecret != nil {
		env = append(env, corev1.EnvVar{
			Name: "MONGODB_URI",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: input.MongoURISecret.Name},
					Key:                  defaultSecretKey(input.MongoURISecret.Key),
				},
			},
		})
	} else {
		env = append(env, corev1.EnvVar{Name: "MONGODB_URI", Value: input.MongoURI})
	}
	return env
}

func deriveJobPhase(job *batchv1.Job) (string, string, *metav1.Time) {
	if job.Status.Succeeded > 0 {
		return phaseSucceeded, "", job.Status.CompletionTime
	}
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			message := cond.Message
			if message == "" {
				message = "job failed"
			}
			return phaseFailed, message, job.Status.CompletionTime
		}
	}
	if job.Status.Active > 0 {
		return phaseRunning, "", job.Status.StartTime
	}
	return phasePending, "", job.Status.StartTime
}

func findActiveJob(ctx context.Context, clientset *kubernetes.Clientset, namespace, seedName string) (string, error) {
	selector := fmt.Sprintf("taskseed.huma.io/name=%s", seedName)
	jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", err
	}
	for _, job := range jobs.Items {
		if job.Status.Active > 0 {
			return job.Name, nil
		}
	}
	return "", nil
}

func patchStatus(ctx context.Context, resource dynamic.ResourceInterface, seed *TaskSeed, status TaskSeedStatus) error {
	patch := map[string]any{"status": status}
	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = resource.Patch(ctx, seed.Name, types.MergePatchType, data, metav1.PatchOptions{}, "status")
	return err
}

func hashSpec(input SeedInput) (string, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func buildJobName(base, hash string) string {
	name := fmt.Sprintf("taskseed-%s-%s", strings.ToLower(base), shortHash(hash))
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "-")
	}
	return name
}

func shortHash(hash string) string {
	if len(hash) < 8 {
		return hash
	}
	return hash[:8]
}

func ownerRef(seed *TaskSeed) metav1.OwnerReference {
	controller := true
	blockDeletion := true
	return metav1.OwnerReference{
		APIVersion:         group + "/" + version,
		Kind:               kind,
		Name:               seed.Name,
		UID:                seed.UID,
		Controller:         &controller,
		BlockOwnerDeletion: &blockDeletion,
	}
}

func int32Ptr(value int32) *int32 {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func defaultSecretKey(key string) string {
	if key == "" {
		return "uri"
	}
	return key
}
