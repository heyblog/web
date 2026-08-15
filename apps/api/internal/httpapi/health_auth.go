package httpapi

func healthAuthorization(expectedToken string) Middleware {
	return BearerAuthorization(expectedToken, "heyblog-health")
}
