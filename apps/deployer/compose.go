package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

const imageNamespace = "sgccr.ccs.tencentyun.com/heyblog"

type composeHost struct {
	composeFile string
	envFile     string
	liveURL     *url.URL
	client      *http.Client
	run         func(context.Context, []string, ...string) (string, error)
}

func newComposeHost(docker, composeFile, envFile string, liveURL *url.URL) composeHost {
	return composeHost{
		composeFile: composeFile,
		envFile:     envFile,
		liveURL:     liveURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		run: func(ctx context.Context, env []string, args ...string) (string, error) {
			cmd := exec.CommandContext(ctx, docker, args...)
			cmd.Env = append(cmd.Environ(), env...)
			output, err := cmd.Output()
			if err != nil {
				return "", errors.Join(err, ctx.Err())
			}
			return strings.TrimSpace(string(output)), nil
		},
	}
}

func (host composeHost) snapshot(ctx context.Context) (images, error) {
	var current images
	for _, service := range []struct {
		name     string
		digest   *string
		optional bool
	}{
		{"api", &current.API, false},
		{"web", &current.Web, false},
		{"worker", &current.Worker, true},
	} {
		container, err := host.compose(ctx, nil, "ps", "--status", "running", "--quiet", service.name)
		if err != nil {
			return images{}, fmt.Errorf("find running %s: %w", service.name, err)
		}
		if container == "" && service.optional {
			continue
		}
		if !isDockerID(container) {
			return images{}, fmt.Errorf("expected one running %s container", service.name)
		}
		imageID, err := host.run(ctx, nil, "container", "inspect", "--format", "{{.Image}}", container)
		if err != nil {
			return images{}, fmt.Errorf("read running %s image ID: %w", service.name, err)
		}
		if !digestPattern.MatchString(imageID) {
			return images{}, fmt.Errorf("invalid running %s image ID", service.name)
		}
		digests, err := host.run(ctx, nil, "image", "inspect", "--format", "{{range .RepoDigests}}{{println .}}{{end}}", imageID)
		if err != nil {
			return images{}, fmt.Errorf("read %s image digests: %w", service.name, err)
		}
		for _, ref := range strings.Fields(digests) {
			digest, ok := strings.CutPrefix(ref, imageNamespace+"/"+service.name+"@")
			if ok && digestPattern.MatchString(digest) {
				*service.digest = digest
				break
			}
		}
		if *service.digest == "" {
			return images{}, fmt.Errorf("running %s image has no HeyBlog CCR digest", service.name)
		}
	}
	return current, nil
}

func isDockerID(id string) bool {
	return len(id) == 64 && digestPattern.MatchString("sha256:"+id)
}

func (host composeHost) pull(ctx context.Context, target images) error {
	_, err := host.compose(ctx, target.environment(), "pull", "api", "web", "worker")
	return err
}

func (host composeHost) apply(ctx context.Context, target images) error {
	services := []string{"api", "web", "worker"}
	env := target.environment()
	if target.Worker == "" {
		// The previous production release may predate the worker.
		if _, err := host.compose(ctx, env, "stop", "worker"); err != nil {
			return fmt.Errorf("stop new worker: %w", err)
		}
		services = services[:2]
	}
	args := []string{"up", "-d", "--no-deps", "--pull", "never", "--wait", "--wait-timeout", "100"}
	if _, err := host.compose(ctx, env, append(args, services...)...); err != nil {
		return fmt.Errorf("start production services and wait for readiness: %w", err)
	}
	return nil
}

func (host composeHost) compose(ctx context.Context, env []string, args ...string) (string, error) {
	base := []string{"compose", "--env-file", host.envFile, "-f", host.composeFile}
	return host.run(ctx, env, append(base, args...)...)
}

func (target images) environment() []string {
	env := []string{
		"API_IMAGE=" + imageNamespace + "/api@" + target.API,
		"WEB_IMAGE=" + imageNamespace + "/web@" + target.Web,
	}
	if target.Worker != "" {
		env = append(env, "WORKER_IMAGE="+imageNamespace+"/worker@"+target.Worker)
	}
	return env
}

func (host composeHost) probe(ctx context.Context, commit string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		err := host.checkHomepage(ctx, commit)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(2 * time.Second):
		}
	}
}

func (host composeHost) checkHomepage(ctx context.Context, commit string) error {
	page := *host.liveURL
	query := page.Query()
	query.Set("deploy_check", fmt.Sprint(time.Now().UnixNano()))
	page.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, page.String(), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Cache-Control", "no-cache")
	response, err := host.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("live homepage returned HTTP %d", response.StatusCode)
	}
	if commit == "" {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if !strings.Contains(string(body), `href="https://github.com/heyblog/web/commit/`+commit+`"`) {
		return fmt.Errorf("live homepage did not report commit %s", commit)
	}
	return nil
}
