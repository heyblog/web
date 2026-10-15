package auth

type bodyInput[T any] struct {
	Body T
}

type emptyInput struct{}

type userResponseBody struct {
	User User `json:"user"`
}

type userOutput struct {
	CacheControl string   `header:"Cache-Control"`
	SetCookie    []string `header:"Set-Cookie"`
	Body         userResponseBody
}

type acceptedResponseBody struct {
	Status string `json:"status" enum:"verification_required"`
}

type acceptedOutput struct {
	Body acceptedResponseBody
}

type noContentOutput struct {
	Status       int      `status:"204"`
	CacheControl string   `header:"Cache-Control"`
	SetCookie    []string `header:"Set-Cookie"`
}

type userPathInput[T any] struct {
	ID   string `path:"id"`
	Body T
}
