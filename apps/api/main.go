package main

import (
	"os"

	"heyblog-api/internal/bootstrap"
)

func main() {
	os.Exit(bootstrap.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
