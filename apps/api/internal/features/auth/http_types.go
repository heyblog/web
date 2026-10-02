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

type usersResponseBody struct {
	Users []User `json:"users"`
}

type usersOutput struct {
	Body usersResponseBody
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

type redirectOutput struct {
	Status       int      `status:"302"`
	Location     string   `header:"Location"`
	CacheControl string   `header:"Cache-Control"`
	SetCookie    []string `header:"Set-Cookie"`
}

type githubStartInput struct {
	Next   string `query:"next"`
	Intent string `query:"intent" enum:"login,bind"`
}

type githubCallbackInput struct {
	Code  string `query:"code" required:"true"`
	State string `query:"state" required:"true"`
}

type userPathInput[T any] struct {
	ID   string `path:"id"`
	Body T
}
