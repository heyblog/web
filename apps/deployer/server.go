package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	deployPath      = "/webhooks/deploy"
	signatureHeader = "X-Zhblogs-Signature-256"
	running         = "running"
	succeeded       = "success"
	failed          = "failed"
	rolledBack      = "rolled_back"
	rollbackFailed  = "rollback_failed"
)

var (
	idPattern     = regexp.MustCompile(`^[0-9]{1,20}-[0-9]{1,10}$`)
	commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type images struct {
	API    string `json:"api"`
	Web    string `json:"web"`
	Worker string `json:"worker"`
}

type release struct {
	ID        string `json:"id"`
	CommitSHA string `json:"commit_sha"`
	Images    images `json:"images"`
}

func (release release) valid() bool {
	return idPattern.MatchString(release.ID) && commitPattern.MatchString(release.CommitSHA) &&
		digestPattern.MatchString(release.Images.API) && digestPattern.MatchString(release.Images.Web) &&
		digestPattern.MatchString(release.Images.Worker)
}

type deployment struct {
	Release  release `json:"release"`
	Status   string  `json:"status"`
	Previous *images `json:"previous,omitempty"`
}

type deploymentHost interface {
	snapshot(context.Context) (images, error)
	pull(context.Context, images) error
	apply(context.Context, images) error
	probe(context.Context, string) error
}

type deployer struct {
	secret   string
	stateDir string
	host     deploymentHost
	mu       sync.Mutex
	active   bool
}

func (d *deployer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "zhblogs-deployer", "api_version": 2})
	})
	mux.HandleFunc("POST "+deployPath, d.submit)
	mux.HandleFunc("GET "+deployPath+"/{id}", d.status)
	return mux
}

func (d *deployer) submit(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2048))
	if err != nil {
		http.Error(w, "Invalid deployment body", http.StatusBadRequest)
		return
	}
	if !d.authorized(r, body) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}
	var target release
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&target); err != nil || decoder.Decode(new(any)) != io.EOF || !target.valid() {
		http.Error(w, "Invalid release", http.StatusBadRequest)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	existing, err := d.read(target.ID)
	switch {
	case err == nil:
		if existing.Release != target {
			http.Error(w, "Deployment ID already in use", http.StatusConflict)
			return
		}
		writeStatus(w, http.StatusOK, existing)
	case !errors.Is(err, os.ErrNotExist):
		d.stateError(w, err)
	case d.active:
		http.Error(w, "Deployment already in progress", http.StatusConflict)
	default:
		job := deployment{Release: target, Status: running}
		if err := d.save(job); err != nil {
			d.stateError(w, err)
			return
		}
		d.active = true
		go d.execute(job)
		writeStatus(w, http.StatusAccepted, job)
	}
}

func (d *deployer) status(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	if !d.authorized(r, []byte("GET "+r.URL.Path)) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}
	job, err := d.read(id)
	switch {
	case errors.Is(err, os.ErrNotExist):
		http.NotFound(w, r)
	case err != nil:
		d.stateError(w, err)
	default:
		writeStatus(w, http.StatusOK, job)
	}
}

func (d *deployer) authorized(r *http.Request, message []byte) bool {
	mac := hmac.New(sha256.New, []byte(d.secret))
	mac.Write(message)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(r.Header.Get(signatureHeader)))
}

func (d *deployer) execute(job deployment) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	d.complete(job.Release.ID, d.deploy(ctx, job))
}

func (d *deployer) complete(id, status string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	job, err := d.read(id)
	if err == nil {
		job.Status = status
		err = d.save(job)
	}
	if err != nil {
		log.Printf("deployment %s: persist final status: %v", id, err)
		return
	}
	log.Printf("deployment %s: %s", id, status)
	d.active = false
}

func (d *deployer) deploy(ctx context.Context, job deployment) string {
	id := job.Release.ID
	previous, err := d.host.snapshot(ctx)
	if err != nil {
		log.Printf("deployment %s: capture running release: %v", id, err)
		return failed
	}
	if err := d.host.pull(ctx, job.Release.Images); err != nil {
		log.Printf("deployment %s: pull new release: %v", id, err)
		return failed
	}
	// Persist the rollback target before the first command that replaces a running container.
	job.Previous = &previous
	if err := d.save(job); err != nil {
		log.Printf("deployment %s: save rollback target: %v", id, err)
		return failed
	}
	err = d.host.apply(ctx, job.Release.Images)
	if err == nil {
		err = d.host.probe(ctx, job.Release.CommitSHA)
	}
	if err == nil {
		return succeeded
	}
	log.Printf("deployment %s: new release did not pass: %v", id, err)
	return d.restore(job)
}

func (d *deployer) restore(job deployment) string {
	if job.Previous == nil {
		return failed
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	err := d.host.apply(ctx, *job.Previous)
	if err == nil {
		err = d.host.probe(ctx, "")
	}
	if err != nil {
		log.Printf("deployment %s: rollback failed: %v", job.Release.ID, err)
		return rollbackFailed
	}
	return rolledBack
}

func (d *deployer) recover() error {
	entries, err := os.ReadDir(d.stateDir)
	if err != nil {
		return err
	}
	var interrupted *deployment
	for _, entry := range entries {
		id, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok || !idPattern.MatchString(id) {
			continue
		}
		job, err := d.read(id)
		if err != nil {
			return err
		}
		if job.Status == running {
			if interrupted != nil {
				return fmt.Errorf("more than one deployment needs recovery")
			}
			interrupted = &job
		}
	}
	if interrupted != nil {
		d.active = true
		log.Printf("deployment %s: recovering interrupted deployment", interrupted.Release.ID)
		go func() {
			d.complete(interrupted.Release.ID, d.restore(*interrupted))
		}()
	}
	return nil
}

func (d *deployer) read(id string) (deployment, error) {
	var job deployment
	data, err := os.ReadFile(filepath.Join(d.stateDir, id+".json"))
	if err != nil {
		return job, err
	}
	if err := json.Unmarshal(data, &job); err != nil {
		return job, err
	}
	if job.Release.ID != id || !job.Release.valid() {
		return job, fmt.Errorf("invalid saved deployment %s", id)
	}
	if previous := job.Previous; previous != nil &&
		(!digestPattern.MatchString(previous.API) || !digestPattern.MatchString(previous.Web) ||
			(previous.Worker != "" && !digestPattern.MatchString(previous.Worker))) {
		return job, fmt.Errorf("invalid rollback images for deployment %s", id)
	}
	return job, nil
}

func (d *deployer) save(job deployment) error {
	file, err := os.CreateTemp(d.stateDir, ".deployment-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	writeErr := json.NewEncoder(file).Encode(job)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), filepath.Join(d.stateDir, job.Release.ID+".json")); err != nil {
		return err
	}
	dir, err := os.Open(d.stateDir)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

func (d *deployer) stateError(w http.ResponseWriter, err error) {
	log.Printf("deployment state: %v", err)
	http.Error(w, "Deployment state unavailable", http.StatusInternalServerError)
}

func writeStatus(w http.ResponseWriter, status int, job deployment) {
	writeJSON(w, status, map[string]string{
		"id": job.Release.ID, "commit_sha": job.Release.CommitSHA, "status": job.Status,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
