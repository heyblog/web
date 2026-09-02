package httpapi

import (
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"

	"heyblog-api/internal/apperror"
	"heyblog-api/internal/application/publicview"
	"heyblog-api/internal/domain/site"
)

var uuidRoutePattern = regexp.MustCompile(
	`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`,
)

func registerPublicViewRoutes(
	router *gin.Engine,
	webToken string,
	reader publicview.Reader,
) error {
	register := func(path string, endpoint Endpoint) error {
		handler, err := adaptApplicationEndpoint(endpointAudienceWeb, webToken, endpoint)
		if err != nil {
			return err
		}
		router.GET(path, handler)
		return nil
	}
	if err := register("/home", func(ctx *Context) (Response, error) {
		view, err := reader.Home(ctx.Request.Context())
		if err != nil {
			return Response{}, err
		}
		response, err := JSON(http.StatusOK, view)
		return response.WithHeader("Cache-Control", "no-store"), err
	}); err != nil {
		return err
	}
	if err := register("/sites", func(ctx *Context) (Response, error) {
		query, err := parseDirectoryQuery(ctx.Request.URL.Query(), time.Now())
		if err != nil {
			return Response{}, err
		}
		view, err := reader.Directory(ctx.Request.Context(), query)
		if err != nil {
			return Response{}, err
		}
		response, err := JSON(http.StatusOK, view)
		return response.WithHeader("Cache-Control", "no-store"), err
	}); err != nil {
		return err
	}
	if err := register("/sites/options", func(ctx *Context) (Response, error) {
		for name := range ctx.Request.URL.Query() {
			return Response{}, invalidDirectoryQuery(name, "is not supported")
		}
		view, err := reader.DirectoryOptions(ctx.Request.Context())
		if err != nil {
			return Response{}, err
		}
		response, err := JSON(http.StatusOK, view)
		return response.WithHeader("Cache-Control", "no-store"), err
	}); err != nil {
		return err
	}
	if err := register("/sites/id/:identifier", func(ctx *Context) (Response, error) {
		identifier, err := parseSiteIdentifier(ctx.Param("identifier"))
		if err != nil {
			return Response{}, err
		}
		view, err := reader.SiteByIdentifier(ctx.Request.Context(), identifier)
		if err != nil {
			return Response{}, err
		}
		response, err := JSON(http.StatusOK, view)
		return response.WithHeader("Cache-Control", "no-store"), err
	}); err != nil {
		return err
	}
	if err := register("/sites/custom/:customId", func(ctx *Context) (Response, error) {
		customID := ctx.Param("customId")
		if err := site.ValidateCustomID(customID); err != nil {
			return Response{}, invalidSiteIdentifier("customId")
		}
		view, err := reader.SiteByCustomID(ctx.Request.Context(), customID)
		if err != nil {
			return Response{}, err
		}
		response, err := JSON(http.StatusOK, view)
		return response.WithHeader("Cache-Control", "no-store"), err
	}); err != nil {
		return err
	}
	return nil
}

func parseSiteIdentifier(value string) (publicview.SiteIdentifier, error) {
	if site.ValidateShortID(value) == nil {
		return publicview.SiteIdentifier{Kind: publicview.IdentifierShortID, Value: value}, nil
	}
	if uuidRoutePattern.MatchString(value) {
		return publicview.SiteIdentifier{Kind: publicview.IdentifierUUID, Value: value}, nil
	}
	return publicview.SiteIdentifier{}, invalidSiteIdentifier("identifier")
}

func invalidSiteIdentifier(name string) error {
	return apperror.New(
		apperror.KindBadRequest,
		apperror.CodeBadRequest,
		"site identifier is invalid",
	).WithInvalidParams([]apperror.InvalidParam{{
		Name: name, Reason: "must use the accepted route format",
	}})
}
