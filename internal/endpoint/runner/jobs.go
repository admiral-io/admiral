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
	artifactRoutePattern = "/api/v1/runner/jobs/{id}/artifact"
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

	return &runnerv1.GetJobBundleResponse{
		Bundle: &runnerv1.JobBundle{
			ArtifactUrl:      artifactURLForJob(jobID),
			Variables:        vars,
			WorkingDirectory: rev.WorkingDirectory,
			// backend_config is empty for 6.3: modules bring their own
			// backend block. Admiral HTTP state backend is a later phase.
			BackendConfig:    "",
			TerraformVersion: "",
		},
	}, nil
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

	// Recompute deployment composite status from the fresh revision set.
	revisions, err := a.revisionStore.ListByDeployment(ctx, rev.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list revisions: %v", err)
	}
	dep, err := a.deploymentStore.Get(ctx, rev.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load deployment: %v", err)
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

	token, err := extractBearer(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	claims, err := a.sessionProvider.Verify(ctx, token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	if claims.Kind != string(authn.TokenKindSAT) {
		http.Error(w, "runner SAT required", http.StatusForbidden)
		return
	}
	runnerID, err := uuid.Parse(claims.Subject)
	if err != nil {
		http.Error(w, "invalid subject in token", http.StatusInternalServerError)
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
