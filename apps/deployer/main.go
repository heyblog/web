package main

import (
	"context"
	"log"
	"net/http"

	"zhblogs-deployer/internal/config"
	"zhblogs-deployer/internal/deploy"
	"zhblogs-deployer/internal/server"
	"zhblogs-deployer/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	rules, err := deploy.LoadRules(cfg.RulesFile)
	if err != nil {
		log.Fatal(err)
	}

	deploymentStore, err := store.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer deploymentStore.Close()

	executor := deploy.NewExecutor(cfg)
	options := server.Options{
		Secret:     cfg.NotifySecret,
		Rules:      rules,
		Store:      deploymentStore,
		Executor:   executor,
		Logger:     log.Default(),
		HTTPClient: http.DefaultClient,
	}
	server.ResumePending(context.Background(), options)
	handler := server.New(options)

	log.Printf("deployer listening on %s", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), handler); err != nil {
		log.Fatal(err)
	}
}
