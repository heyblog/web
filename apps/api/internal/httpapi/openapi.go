package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	"github.com/swaggest/swgui/v5emb"

	"heyblog-api/internal/config"
)

type Router struct {
	*gin.Engine
	API huma.API
}

func newContractAPI(engine *gin.Engine, logger *slog.Logger) huma.API {
	jsonFormat := huma.Format{
		Marshal: func(writer io.Writer, value any) error {
			body, err := json.Marshal(value)
			if err != nil {
				return err
			}
			_, err = writer.Write(body)
			return err
		},
		Unmarshal: func(data []byte, value any) error {
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(value); err != nil {
				return err
			}
			var extra any
			if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
				if err == nil {
					return errors.New("multiple JSON documents are not allowed")
				}
				return err
			}
			return nil
		},
	}
	contract := huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI: "3.1.0",
			Info: &huma.Info{
				Title:   "HeyBlog API",
				Version: "1.0.0",
			},
			Components: &huma.Components{SecuritySchemes: securitySchemes()},
		},
		Formats: map[string]huma.Format{
			"application/json":         jsonFormat,
			"application/problem+json": jsonFormat,
		},
		DefaultFormat:                "application/json",
		RejectUnknownQueryParameters: false,
	}
	api := humagin.New(engine, contract)
	api.UseMiddleware(attachRequestContext(logger))
	return api
}

func securitySchemes() map[string]*huma.SecurityScheme {
	return map[string]*huma.SecurityScheme{
		"webToken": {
			Type: "apiKey",
			Name: WebTokenHeader,
			In:   "header",
		},
		"healthBearer": {
			Type:   "http",
			Scheme: "bearer",
		},
		"importBearer": {
			Type:   "http",
			Scheme: "bearer",
		},
		"accessCookie": {
			Type: "apiKey",
			Name: "heyblog_access_token",
			In:   "cookie",
		},
		"refreshCookie": {
			Type: "apiKey",
			Name: "heyblog_refresh_token",
			In:   "cookie",
		},
	}
}

func registerOpenAPIRoutes(router *Router, mode config.Mode) {
	if mode != config.ModeDevelopment {
		return
	}
	router.GET("/openapi.json", func(ctx *gin.Context) {
		body, err := json.Marshal(router.API.OpenAPI())
		if err != nil {
			_ = ctx.Error(err)
			ctx.Abort()
			return
		}
		ctx.Data(http.StatusOK, "application/openapi+json", body)
	})
	router.GET("/openapi.yaml", func(ctx *gin.Context) {
		body, err := router.API.OpenAPI().YAML()
		if err != nil {
			_ = ctx.Error(err)
			ctx.Abort()
			return
		}
		ctx.Data(http.StatusOK, "application/openapi+yaml", body)
	})
	swaggerUI := v5emb.New("HeyBlog API", "/openapi.json", "/swagger/")
	serveSwaggerUI := func(ctx *gin.Context) {
		ctx.Header("Content-Security-Policy", "default-src 'none'; base-uri 'none'; connect-src 'self'; font-src 'self'; form-action 'none'; frame-ancestors 'none'; img-src 'self' data:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
		swaggerUI.ServeHTTP(ctx.Writer, ctx.Request)
	}
	router.GET("/swagger", serveSwaggerUI)
	router.GET("/swagger/*asset", serveSwaggerUI)
}
