package runner

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/backend"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/querybuilder"
	"go.admiral.io/admiral/internal/service/authn"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

const (
	artifactRoutePattern  = "/api/v1/runner/jobs/{id}/artifact"
	planFileRoutePattern  = "/api/v1/runner/jobs/{id}/plan"
	planFileContentType   = "application/octet-stream"
	maxPlanFileSize       = 256 << 20 // 256 MiB
)

func (a *api) ClaimJob(ctx context.Context, _ *runnerv1.ClaimJobRequest) (*runnerv1.ClaimJobResponse, error) {
	runnerID, err := runnerIDFromClaims(ctx)
	if err != nil {
		return nil, err
	}

	r, err := a.store.Get(ctx, runnerID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	job, err := a.jobStore.ClaimNextJob(ctx, runnerID, r.LastInstanceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to claim job: %v", err)
	}

	resp := &runnerv1.ClaimJobResponse{}
	if job != nil {
		resp.Job = job.ToProto()
	}
	return resp, nil
}

func (a *api) GetJobBundle(ctx context.Context, req *runnerv1.GetJobBundleRequest) (*runnerv1.GetJobBundleResponse, error) {
	runnerID, err := runnerIDFromClaims(ctx)
	if err != nil {
		return nil, err
	}

	jobID, err := uuid.Parse(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid job_id: %v", err)
	}

	job, err := a.jobStore.Get(ctx, jobID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %s", jobID)
	}
	if job.RunnerId != runnerID {
		return nil, status.Errorf(codes.PermissionDenied, "job does not belong to this runner")
	}

	rev, err := a.revisionStore.Get(ctx, job.RevisionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load revision: %v", err)
	}

	vars, err := model.ParseResolvedValuesAsVars(rev.ResolvedValues)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse resolved values: %v", err)
	}

	bundle := &runnerv1.JobBundle{
		ArtifactUrl:      artifactURLForJob(jobID),
		Variables:        vars,
		WorkingDirectory: rev.WorkingDirectory,
		// backend_config is empty for 6.3: modules bring their own
		// backend block. Admiral HTTP state backend is a later phase.
		BackendConfig:    "",
		TerraformVersion: "",
	}

	// For apply jobs, include the URL to download the binary plan file
	// produced by the preceding plan job.
	if job.JobType == model.JobTypeApply || job.JobType == model.JobTypeDestroyApply {
		if rev.PlanFileKey != "" {
			bundle.PlanFileUrl = planFileURLForJob(jobID)
		}
	}

	return &runnerv1.GetJobBundleResponse{Bundle: bundle}, nil
}

func (a *api) ReportJobResult(ctx context.Context, req *runnerv1.ReportJobResultRequest) (*runnerv1.ReportJobResultResponse, error) {
	runnerID, err := runnerIDFromClaims(ctx)
	if err != nil {
		return nil, err
	}

	jobID, err := uuid.Parse(req.GetJobId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid job_id: %v", err)
	}

	job, err := a.jobStore.Get(ctx, jobID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %s", jobID)
	}
	if job.RunnerId != runnerID {
		return nil, status.Errorf(codes.PermissionDenied, "job does not belong to this runner")
	}

	result := req.GetResult()
	if result == nil {
		return nil, status.Error(codes.InvalidArgument, "result is required")
	}

	jobStatus, err := model.JobStatusFromProto(result.GetStatus())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	now := time.Now()

	if _, err := a.jobStore.Update(ctx, job, map[string]any{
		"status":       jobStatus,
		"completed_at": now,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update job: %v", err)
	}

	rev, err := a.revisionStore.Get(ctx, job.RevisionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load revision: %v", err)
	}

	revFields := model.DeriveRevisionUpdate(job.JobType, jobStatus, result)
	revFields["completed_at"] = now

	// Persist plan output to object storage.
	if planOutput := result.GetPlanOutput(); planOutput != "" {
		key := fmt.Sprintf("plans/%s/plan.txt", rev.Id)
		if err := a.objStore.PutObject(ctx, a.objBucket, key, []byte(planOutput)); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to write plan output to object storage: %v", err)
		}
		revFields["plan_output_key"] = key
	}

	if _, err := a.revisionStore.Update(ctx, rev, revFields); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update revision: %v", err)
	}

	// Capture Terraform outputs after a successful apply.
	dep, err := a.deploymentStore.Get(ctx, rev.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load deployment: %v", err)
	}

	if jobStatus == model.JobStatusSucceeded {
		if err := a.captureOutputs(ctx, job, rev, dep, result); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to capture outputs: %v", err)
		}

		// Promote blocked jobs whose dependencies are now satisfied.
		if err := a.promoteUnblockedJobs(ctx, rev); err != nil {
			a.logger.Error("failed to promote unblocked jobs",
				zap.String("deployment_id", rev.DeploymentId.String()),
				zap.String("component", rev.ComponentName),
				zap.Error(err))
		}
	}

	// Recompute deployment composite status from the fresh revision set.
	revisions, err := a.revisionStore.ListByDeployment(ctx, rev.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list revisions: %v", err)
	}
	depStatus := model.DeriveDeploymentStatus(revisions)
	depFields := map[string]any{
		"status":     depStatus,
		"updated_at": now,
	}
	if model.IsTerminalDeploymentStatus(depStatus) {
		depFields["completed_at"] = now
	}
	if _, err := a.deploymentStore.Update(ctx, dep, depFields); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update deployment: %v", err)
	}

	return &runnerv1.ReportJobResultResponse{Ack: true}, nil
}

// captureOutputs persists Terraform outputs as infrastructure variables after
// a successful apply, or deletes them after a successful destroy.
func (a *api) captureOutputs(
	ctx context.Context,
	job *model.Job,
	rev *model.Revision,
	dep *model.Deployment,
	result *runnerv1.JobResult,
) error {
	switch job.JobType {
	case model.JobTypeApply:
		outputs := result.GetOutputs()
		if len(outputs) == 0 {
			return nil
		}
		vars := model.VariablesFromTerraformOutputs(
			outputs,
			rev.ComponentName,
			dep.ApplicationId,
			dep.EnvironmentId,
			"system:output-capture",
		)
		if err := a.variableStore.UpsertInfraOutputs(
			ctx, dep.ApplicationId, dep.EnvironmentId,
			rev.ComponentName, vars,
		); err != nil {
			a.logger.Error("output capture: upsert failed",
				zap.String("component", rev.ComponentName),
				zap.String("deployment_id", dep.Id.String()),
				zap.Error(err))
			return err
		}
		a.logger.Info("output capture: stored infrastructure outputs",
			zap.String("component", rev.ComponentName),
			zap.Int("count", len(vars)))

	case model.JobTypeDestroyApply:
		if err := a.variableStore.DeleteInfraOutputs(
			ctx, dep.ApplicationId, dep.EnvironmentId,
			rev.ComponentName,
		); err != nil {
			a.logger.Error("output capture: delete failed",
				zap.String("component", rev.ComponentName),
				zap.String("deployment_id", dep.Id.String()),
				zap.Error(err))
			return err
		}
		a.logger.Info("output capture: cleared infrastructure outputs after destroy",
			zap.String("component", rev.ComponentName))
	}
	return nil
}

// promoteUnblockedJobs checks for PENDING jobs in the same deployment whose
// BlockedBy dependencies are now all satisfied (their revisions have reached
// a post-plan or post-apply terminal state). Satisfied jobs are promoted from
// PENDING to ASSIGNED so the runner can claim them.
func (a *api) promoteUnblockedJobs(ctx context.Context, completedRev *model.Revision) error {
	pendingJobs, err := a.jobStore.ListByDeploymentAndStatus(ctx, completedRev.DeploymentId, model.JobStatusPending)
	if err != nil {
		return fmt.Errorf("list pending jobs: %w", err)
	}
	if len(pendingJobs) == 0 {
		return nil
	}

	// Load all revisions to build a component-name → status lookup.
	revisions, err := a.revisionStore.ListByDeployment(ctx, completedRev.DeploymentId)
	if err != nil {
		return fmt.Errorf("list revisions: %w", err)
	}
	revByComponent := make(map[string]*model.Revision, len(revisions))
	revByID := make(map[uuid.UUID]*model.Revision, len(revisions))
	for i := range revisions {
		revByComponent[revisions[i].ComponentName] = &revisions[i]
		revByID[revisions[i].Id] = &revisions[i]
	}

	for i := range pendingJobs {
		pj := &pendingJobs[i]
		rev, ok := revByID[pj.RevisionId]
		if !ok {
			continue
		}
		if len(rev.BlockedBy) == 0 {
			continue
		}

		allSatisfied := true
		for _, blockerName := range rev.BlockedBy {
			blocker, ok := revByComponent[blockerName]
			if !ok {
				allSatisfied = false
				break
			}
			if !model.IsRevisionSatisfiedFor(pj.JobType, blocker.Status) {
				allSatisfied = false
				break
			}
		}

		if allSatisfied {
			if err := a.jobStore.PromoteToAssigned(ctx, pj.Id); err != nil {
				a.logger.Error("failed to promote job",
					zap.String("job_id", pj.Id.String()),
					zap.Error(err))
				continue
			}
			a.logger.Info("promoted blocked job",
				zap.String("job_id", pj.Id.String()),
				zap.String("component", rev.ComponentName))
		}
	}
	return nil
}

func (a *api) ListRunnerJobs(ctx context.Context, req *runnerv1.ListRunnerJobsRequest) (*runnerv1.ListRunnerJobsResponse, error) {
	runnerID, err := uuid.Parse(req.GetRunnerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}
	if _, err := a.store.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	var pageToken *string
	if req.GetPageToken() != "" {
		pt := req.GetPageToken()
		pageToken = &pt
	}

	jobs, err := a.jobStore.ListByRunner(
		ctx,
		runnerID,
		a.jobsQB.PaginatedQuery(req.GetFilter(), req.GetPageSize(), pageToken),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list jobs: %v", err)
	}

	resp := &runnerv1.ListRunnerJobsResponse{}
	for i := range jobs {
		resp.Jobs = append(resp.Jobs, jobs[i].ToProto())
	}
	if len(jobs) > 0 && int32(len(jobs)) == querybuilder.EffectiveLimit(req.GetPageSize()) {
		last := jobs[len(jobs)-1]
		token := fmt.Sprintf("%d|%s", last.CreatedAt.Unix(), last.Id.String())
		resp.NextPageToken = base64.RawURLEncoding.EncodeToString([]byte(token))
	}
	return resp, nil
}

func (a *api) serveArtifact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	runnerID, err := a.authenticateRunner(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	jobID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}

	job, err := a.jobStore.Get(ctx, jobID)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	if job.RunnerId != runnerID {
		http.Error(w, "job does not belong to this runner", http.StatusForbidden)
		return
	}

	rev, err := a.revisionStore.Get(ctx, job.RevisionId)
	if err != nil {
		a.logger.Error("artifact: load revision failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	comp, err := a.componentStore.Get(ctx, rev.ComponentId)
	if err != nil {
		a.logger.Error("artifact: load component failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	mod, err := a.moduleStore.Get(ctx, comp.ModuleId)
	if err != nil {
		a.logger.Error("artifact: load module failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	src, err := a.sourceStore.Get(ctx, mod.SourceId)
	if err != nil {
		a.logger.Error("artifact: load source failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var cred *model.Credential
	if src.CredentialId != nil {
		cred, err = a.credentialStore.Get(ctx, *src.CredentialId)
		if err != nil {
			a.logger.Error("artifact: load credential failed", zap.Error(err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	be, err := backend.For(src.Type)
	if err != nil {
		a.logger.Error("artifact: no backend for source type", zap.String("type", src.Type), zap.Error(err))
		http.Error(w, "no backend for source", http.StatusInternalServerError)
		return
	}

	ref := firstNonEmpty(rev.Version, mod.Ref)
	fetch, err := be.Fetch(ctx, cred, src, backend.FetchOptions{
		Ref:  ref,
		Root: mod.Root,
	})
	if err != nil {
		a.logger.Error("artifact: fetch failed",
			zap.String("job_id", jobID.String()),
			zap.String("source_id", src.Id.String()),
			zap.Error(err))
		http.Error(w, "fetch failed", http.StatusBadGateway)
		return
	}
	defer fetch.Cleanup()

	if mod.Path != "" {
		pathDir := filepath.Join(fetch.Dir, mod.Path)
		info, err := os.Stat(pathDir)
		if err != nil || !info.IsDir() {
			a.logger.Error("artifact: module path not found",
				zap.String("path", mod.Path), zap.Error(err))
			http.Error(w, "module path not found in fetched content", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s.tar.gz"`, jobID))

	if err := writeTarGz(w, fetch.Dir); err != nil {
		a.logger.Error("artifact: tar write failed",
			zap.String("job_id", jobID.String()), zap.Error(err))
		// Headers already written; log-and-return.
		return
	}
}

func artifactURLForJob(jobID uuid.UUID) string {
	return fmt.Sprintf("/api/v1/runner/jobs/%s/artifact", jobID)
}

func planFileURLForJob(jobID uuid.UUID) string {
	return fmt.Sprintf("/api/v1/runner/jobs/%s/plan", jobID)
}

// uploadPlanFile handles POST /api/v1/runner/jobs/{id}/plan.
// The runner uploads the binary .tfplan file after a successful plan job.
func (a *api) uploadPlanFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	runnerID, err := a.authenticateRunner(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	jobID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}

	job, err := a.jobStore.Get(ctx, jobID)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	if job.RunnerId != runnerID {
		http.Error(w, "job does not belong to this runner", http.StatusForbidden)
		return
	}

	if job.JobType != model.JobTypePlan && job.JobType != model.JobTypeDestroyPlan {
		http.Error(w, "plan file upload is only valid for plan jobs", http.StatusBadRequest)
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxPlanFileSize)
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		a.logger.Error("plan file: read body failed", zap.String("job_id", jobID.String()), zap.Error(err))
		http.Error(w, "failed to read plan file", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "empty plan file", http.StatusBadRequest)
		return
	}

	rev, err := a.revisionStore.Get(ctx, job.RevisionId)
	if err != nil {
		a.logger.Error("plan file: load revision failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	key := fmt.Sprintf("plans/%s/plan.tfplan", rev.Id)
	if err := a.objStore.PutObject(ctx, a.objBucket, key, data); err != nil {
		a.logger.Error("plan file: write to object storage failed",
			zap.String("key", key), zap.Error(err))
		http.Error(w, "failed to store plan file", http.StatusInternalServerError)
		return
	}

	if _, err := a.revisionStore.Update(ctx, rev, map[string]any{
		"plan_file_key": key,
	}); err != nil {
		a.logger.Error("plan file: update revision failed", zap.Error(err))
		http.Error(w, "failed to update revision", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// downloadPlanFile handles GET /api/v1/runner/jobs/{id}/plan.
// The runner downloads the binary .tfplan file before executing an apply job.
func (a *api) downloadPlanFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	runnerID, err := a.authenticateRunner(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	jobID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}

	job, err := a.jobStore.Get(ctx, jobID)
	if err != nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	if job.RunnerId != runnerID {
		http.Error(w, "job does not belong to this runner", http.StatusForbidden)
		return
	}

	rev, err := a.revisionStore.Get(ctx, job.RevisionId)
	if err != nil {
		a.logger.Error("plan file download: load revision failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if rev.PlanFileKey == "" {
		http.Error(w, "no plan file available for this revision", http.StatusNotFound)
		return
	}

	data, err := a.objStore.GetObject(ctx, a.objBucket, rev.PlanFileKey)
	if err != nil {
		a.logger.Error("plan file download: read from object storage failed",
			zap.String("key", rev.PlanFileKey), zap.Error(err))
		http.Error(w, "failed to read plan file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", planFileContentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s.tfplan"`, rev.Id))
	w.Write(data)
}

// authenticateRunner extracts and verifies the runner SAT from the request,
// returning the runner's UUID. Used by raw HTTP handlers that bypass gRPC.
func (a *api) authenticateRunner(r *http.Request) (uuid.UUID, error) {
	token, err := extractBearer(r.Header.Get("Authorization"))
	if err != nil {
		return uuid.Nil, err
	}
	claims, err := a.sessionProvider.Verify(r.Context(), token)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid token")
	}
	if claims.Kind != string(authn.TokenKindSAT) {
		return uuid.Nil, fmt.Errorf("runner SAT required")
	}
	return uuid.Parse(claims.Subject)
}

func extractBearer(header string) (string, error) {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return "", errors.New("missing or malformed Authorization header")
	}
	if fields[1] == "" {
		return "", errors.New("empty bearer token")
	}
	return fields[1], nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func writeTarGz(w io.Writer, root string) error {
	gz := gzip.NewWriter(w)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip .git and other VCS metadata -- not needed for terraform
		// execution and inflates the bundle considerably for git backends.
		if info.IsDir() && (info.Name() == ".git" || info.Name() == ".terraform") {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", path, err)
			}
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
