package taskcenter

import (
	"context"
	"errors"
	"net/url"

	"heyblog-api/internal/domain/proof"
	"heyblog-api/internal/platform/httpapi"

	"github.com/danielgtaylor/huma/v2"
)

type generateInput struct {
	KeyInput
	Body struct {
		Platform proof.Platform `json:"platform" enum:"CF,EO"`
	}
}
type verifyInput struct {
	Body struct {
		Token  string `json:"token" maxLength:"8192"`
		Method string `json:"method,omitempty" enum:",GET,HEAD"`
		URL    string `json:"url,omitempty" maxLength:"16384"`
	}
}

func registerSigningRoutes(api huma.API, r Routes) {
	httpapi.Register(api, r.operation("get-request-signing-policy", "GET", "request-signing/policy"), func(ctx context.Context, _ *emptyInput) (*output[Policy], error) {
		if _, err := r.authorize(ctx, true); err != nil {
			return nil, err
		}
		return response(r.Signing.Policy(ctx))
	})
	httpapi.Register(api, r.operation("set-request-signing-policy", "PUT", "request-signing/policy"), func(ctx context.Context, in *bodyInput[Policy]) (*output[Policy], error) {
		return signingMutation(ctx, r, in.Key, "policy", in.Body, func(signing *Signing, actor string) (Policy, error) {
			if err := signing.SetPolicy(ctx, actor, in.Body); err != nil {
				return Policy{}, err
			}
			return signing.Policy(ctx)
		})
	})
	httpapi.Register(api, r.operation("list-request-signing-keys", "GET", "request-signing/keys"), func(ctx context.Context, _ *emptyInput) (*output[[]SigningKey], error) {
		if _, err := r.authorize(ctx, true); err != nil {
			return nil, err
		}
		return response(r.Signing.Keys(ctx, false))
	})
	httpapi.Register(api, r.operation("generate-request-signing-key", "POST", "request-signing/keys"), func(ctx context.Context, in *generateInput) (*output[accepted], error) {
		return signingMutation(ctx, r, in.Key, "generate", in.Body, func(signing *Signing, actor string) (accepted, error) {
			id, err := signing.Generate(ctx, actor, in.Body.Platform)
			return accepted{id}, err
		})
	})
	for _, action := range []string{"publish", "activate", "revoke"} {
		httpapi.Register(api, r.operation(action+"-request-signing-key", "POST", "request-signing/keys/{id}/"+action), func(ctx context.Context, in *controlInput) (*output[accepted], error) {
			return signingMutation(ctx, r, in.Key, action+":"+in.ID, in.ID, func(signing *Signing, actor string) (accepted, error) {
				return accepted{in.ID}, signing.Transition(ctx, actor, in.ID, action)
			})
		})
	}
	httpapi.Register(api, r.operation("list-request-signing-events", "GET", "request-signing/events"), func(ctx context.Context, _ *emptyInput) (*output[[]SigningEvent], error) {
		if _, err := r.authorize(ctx, true); err != nil {
			return nil, err
		}
		return response(r.Signing.Events(ctx))
	})
	public := func(id, method, path string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: "/api/web/public/request-signing/" + path, Middlewares: huma.Middlewares{httpapi.HumaWebAuthorization(r.WebToken)}, Errors: []int{401, 422, 500}}
	}
	httpapi.Register(api, public("public-request-signing-keys", "GET", "keys"), func(ctx context.Context, _ *emptyInput) (*output[[]SigningKey], error) {
		return response(r.Signing.Keys(ctx, true))
	})
	httpapi.Register(api, public("verify-request-signature", "POST", "verify"), func(ctx context.Context, in *verifyInput) (*output[ProofCheck], error) {
		if (in.Body.Method == "") != (in.Body.URL == "") {
			return nil, invalid(errors.New("method and URL must be supplied together"))
		}
		if in.Body.URL != "" {
			u, err := url.Parse(in.Body.URL)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
				return nil, invalid(errors.New("invalid observed URL"))
			}
		}
		check, err := r.Signing.Verify(ctx, in.Body.Token)
		check.Binding = "unbound"
		if check.Status == "valid" && check.Claims != nil && in.Body.Method != "" {
			check.Binding = "mismatch"
			if check.Claims.Method == in.Body.Method && check.Claims.URL == in.Body.URL {
				check.Binding = "matched"
			}
		}
		return response(check, err)
	})
}

func signingMutation[T any](ctx context.Context, r Routes, key, operation string, payload any, fn func(*Signing, string) (T, error)) (*output[T], error) {
	user, err := r.authorize(ctx, true)
	if err != nil {
		return nil, err
	}
	digest, err := payloadDigest(payload)
	if err != nil {
		return nil, err
	}
	return response(mutate(ctx, r.Repository, mutation[T]{Scope: user.ID + ":signing:" + operation, Key: key, Digest: digest},
		func(repo *Repository) (T, error) { return fn(NewSigning(repo, r.Signing.master), user.ID) }))
}
