package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	oldImages  = images{"sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("b", 64), ""}
	nextImages = images{"sha256:" + strings.Repeat("d", 64), "sha256:" + strings.Repeat("e", 64), "sha256:" + strings.Repeat("f", 64)}
	next       = release{ID: "123-1", CommitSHA: strings.Repeat("1", 40), Images: nextImages}
)

type fakeHost struct {
	applied       []images
	probed        []string
	pullError     error
	newProbeError error
	rollbackError error
}

func (host *fakeHost) snapshot(context.Context) (images, error) { return oldImages, nil }

func (host *fakeHost) pull(context.Context, images) error { return host.pullError }

func (host *fakeHost) apply(_ context.Context, target images) error {
	host.applied = append(host.applied, target)
	if target == oldImages {
		return host.rollbackError
	}
	return nil
}

func (host *fakeHost) probe(_ context.Context, expectedCommit string) error {
	host.probed = append(host.probed, expectedCommit)
	if expectedCommit != "" {
		return host.newProbeError
	}
	return nil
}

func TestDeployAndRollback(t *testing.T) {
	unhealthy := errors.New("unhealthy")
	for _, test := range []struct {
		name    string
		host    fakeHost
		status  string
		applied []images
		probed  []string
	}{
		{"healthy", fakeHost{}, succeeded, []images{nextImages}, []string{next.CommitSHA}},
		{"pull failed", fakeHost{pullError: unhealthy}, failed, nil, nil},
		{"live check failed", fakeHost{newProbeError: unhealthy}, rolledBack, []images{nextImages, oldImages}, []string{next.CommitSHA, ""}},
		{"rollback failed", fakeHost{newProbeError: unhealthy, rollbackError: unhealthy}, rollbackFailed, []images{nextImages, oldImages}, []string{next.CommitSHA}},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := &deployer{stateDir: t.TempDir(), host: &test.host}
			job := deployment{Release: next, Status: running}
			if got := d.deploy(t.Context(), job); got != test.status {
				t.Fatalf("status %q, want %q", got, test.status)
			}
			if !reflect.DeepEqual(test.host.applied, test.applied) || !reflect.DeepEqual(test.host.probed, test.probed) {
				t.Fatalf("applied %v, probed %v", test.host.applied, test.host.probed)
			}
			if len(test.applied) > 0 {
				saved, err := d.read(next.ID)
				if err != nil || saved.Previous == nil || *saved.Previous != oldImages {
					t.Fatalf("rollback journal has wrong images: %+v, %v", saved, err)
				}
			}
		})
	}
}

func TestRecoverAnInterruptedDeployment(t *testing.T) {
	host := &fakeHost{}
	d := &deployer{stateDir: t.TempDir(), host: host}
	if err := d.save(deployment{Release: next, Status: running, Previous: &oldImages}); err != nil {
		t.Fatal(err)
	}
	if err := d.recover(); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		d.mu.Lock()
		active := d.active
		d.mu.Unlock()
		if !active {
			job, err := d.read(next.ID)
			if err != nil || job.Status != rolledBack || !reflect.DeepEqual(host.applied, []images{oldImages}) {
				t.Fatalf("recovery returned %+v, %v after applying %v", job, err, host.applied)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("recovery did not reach rolled_back")
}

func TestSignedWebhook(t *testing.T) {
	d := &deployer{secret: "local-test-signature", stateDir: t.TempDir(), host: &fakeHost{}}
	handler := d.routes()
	send := func(method, path string, data []byte, signed bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		if signed {
			message := data
			if method == http.MethodGet {
				message = []byte("GET " + path)
			}
			mac := hmac.New(sha256.New, []byte(d.secret))
			mac.Write(message)
			r.Header.Set(signatureHeader, "sha256="+hex.EncodeToString(mac.Sum(nil)))
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body, err := json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	if got := send(http.MethodPost, deployPath, body, false); got.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned submission returned %d", got.Code)
	}
	if got := send(http.MethodPost, deployPath, []byte(`{"id":"123-1","images":{"api":"latest"}}`), true); got.Code != http.StatusBadRequest {
		t.Fatalf("non-digest submission returned %d", got.Code)
	}
	if got := send(http.MethodPost, deployPath, body, true); got.Code != http.StatusAccepted {
		t.Fatalf("valid submission returned %d, %s", got.Code, got.Body)
	}
	if got := send(http.MethodPost, deployPath, body, true); got.Code != http.StatusOK {
		t.Fatalf("identical retry returned %d", got.Code)
	}
	statusPath := deployPath + "/" + next.ID
	if got := send(http.MethodGet, statusPath, nil, false); got.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned status request returned %d", got.Code)
	}
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		got := send(http.MethodGet, statusPath, nil, true)
		if got.Code != http.StatusOK {
			t.Fatalf("signed status returned %d, %s", got.Code, got.Body)
		}
		if strings.Contains(got.Body.String(), `"status":"success"`) {
			d.mu.Lock()
			d.mu.Unlock()
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("deployment did not reach success")
}
