package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestComposeCapturesRunningDigests(t *testing.T) {
	id := strings.Repeat("0", 64)
	host := composeHost{
		composeFile: "/prod/compose.yaml",
		envFile:     "/prod/.env",
		run: func(_ context.Context, _ []string, args ...string) (string, error) {
			switch args[0] {
			case "compose":
				if args[len(args)-1] == "worker" {
					return "", nil
				}
				return id, nil
			case "container":
				return "sha256:" + id, nil
			case "image":
				return imageNamespace + "/api@" + oldImages.API + "\n" +
					imageNamespace + "/web@" + oldImages.Web, nil
			}
			return "", fmt.Errorf("unexpected Docker arguments %v", args)
		},
	}
	if got, err := host.snapshot(t.Context()); err != nil || got != oldImages {
		t.Fatalf("captured %+v, %v", got, err)
	}
}

func TestComposeRestoresOlderReleaseWithoutWorker(t *testing.T) {
	var commands [][]string
	host := composeHost{
		composeFile: "/prod/compose.yaml",
		envFile:     "/prod/.env",
		run: func(_ context.Context, env []string, args ...string) (string, error) {
			wantEnv := []string{
				"API_IMAGE=" + imageNamespace + "/api@" + oldImages.API,
				"WEB_IMAGE=" + imageNamespace + "/web@" + oldImages.Web,
			}
			if !reflect.DeepEqual(env, wantEnv) {
				t.Errorf("Docker environment %v, want %v", env, wantEnv)
			}
			if !reflect.DeepEqual(args[:5], []string{"compose", "--env-file", "/prod/.env", "-f", "/prod/compose.yaml"}) {
				t.Errorf("Docker used a different project: %v", args)
			}
			commands = append(commands, args[5:])
			return "", nil
		},
	}
	if err := host.apply(t.Context(), oldImages); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"stop", "worker"},
		{"up", "-d", "--no-deps", "--pull", "never", "--wait", "--wait-timeout", "100", "api", "web"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("Compose commands %v, want %v", commands, want)
	}
}

func TestHomepageMustServeThePublishedCommit(t *testing.T) {
	site := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("deploy_check") == "" || r.Header.Get("Cache-Control") != "no-cache" {
			t.Error("homepage check must bypass cached responses")
		}
		fmt.Fprintf(w, `<a href="https://github.com/heyblog/web/commit/%s">Build</a>`, next.CommitSHA)
	}))
	defer site.Close()
	liveURL, err := url.Parse(site.URL)
	if err != nil {
		t.Fatal(err)
	}
	host := composeHost{client: site.Client(), liveURL: liveURL}
	if err := host.checkHomepage(t.Context(), next.CommitSHA); err != nil {
		t.Fatal(err)
	}
	if err := host.checkHomepage(t.Context(), strings.Repeat("2", 40)); err == nil {
		t.Fatal("homepage for another version passed the deployment check")
	}
}
