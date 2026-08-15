package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var errInvalidResponse = errors.New("endpoint returned an invalid success response")

// Context exposes request data needed by endpoints without exposing Gin's
// response writer. Responses remain owned by Adapt and the error boundary.
type Context struct {
	Request *http.Request
	native  *gin.Context
}

type Response struct {
	status      int
	contentType string
	headers     http.Header
	body        []byte
}

type Endpoint func(*Context) (Response, error)

type Middleware func(Endpoint) Endpoint

func JSON(status int, value any) (Response, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return Response{}, err
	}
	return Response{status: status, contentType: "application/json; charset=utf-8", body: body}, nil
}

func NoContent(status int) Response {
	return Response{status: status}
}

func (response Response) WithHeader(key, value string) Response {
	if response.headers == nil {
		response.headers = make(http.Header)
	}
	response.headers.Add(key, value)
	return response
}

func Chain(endpoint Endpoint, middleware ...Middleware) Endpoint {
	for index := len(middleware) - 1; index >= 0; index-- {
		endpoint = middleware[index](endpoint)
	}
	return endpoint
}

func Adapt(endpoint Endpoint) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		response, err := endpoint(&Context{Request: ctx.Request, native: ctx})
		if err != nil {
			_ = ctx.Error(err)
			ctx.Abort()
			return
		}
		if response.status < http.StatusOK || response.status > 599 {
			_ = ctx.Error(errInvalidResponse)
			ctx.Abort()
			return
		}
		response.write(ctx)
	}
}

func (ctx *Context) ClientIP() string {
	return ctx.native.ClientIP()
}

func (ctx *Context) Header(key, value string) {
	ctx.native.Header(key, value)
}

func (ctx *Context) SetReadDeadline(deadline time.Time) error {
	return http.NewResponseController(ctx.native.Writer).SetReadDeadline(deadline)
}

func (ctx *Context) SetWriteDeadline(deadline time.Time) error {
	return http.NewResponseController(ctx.native.Writer).SetWriteDeadline(deadline)
}

func (response Response) write(ctx *gin.Context) {
	for key, values := range response.headers {
		for _, value := range values {
			ctx.Writer.Header().Add(key, value)
		}
	}
	if response.contentType != "" {
		ctx.Data(response.status, response.contentType, response.body)
		return
	}
	ctx.Status(response.status)
}
