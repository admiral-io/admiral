package runner

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/backend"
	"go.admiral.io/admiral/internal/model"
)

const (
	artifactRoutePattern = "/api/v1/runner/jobs/{id}/artifact"
	planFileRoutePattern = "/api/v1/runner/jobs/{id}/plan"
	planFileContentType  = "application/octet-stream"
	maxPlanFileSize      = 256 << 20 // 256 MiB
)

func artifactURLForJob(jobID uuid.UUID) string {
	return fmt.Sprintf("/api/v1/runner/jobs/%s/artifact", jobID)
}

func planFileURLForJob(jobID uuid.UUID) string {
	return fmt.Sprintf("/api/v1/runner/jobs/%s/plan", jobID)
}

func (a *api) serveArtifact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	runnerID, err := runnerIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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
		pathDir := filepath.Join(fetch.Dir, fetch.WorkingDirectory, mod.Path)
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

func (a *api) uploadPlanFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	runnerID, err := runnerIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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

func (a *api) downloadPlanFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	runnerID, err := runnerIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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
	if _, err := w.Write(data); err != nil {
		a.logger.Warn("failed to write plan file response",
			zap.String("revision_id", rev.Id.String()),
			zap.Error(err))
	}
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
