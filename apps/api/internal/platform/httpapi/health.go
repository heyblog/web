package httpapi

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

var errDraining = errors.New("service is draining")

type Readiness interface {
	Ready(context.Context) error
}

type Health struct {
	readiness Readiness
	timeout   time.Duration
	draining  atomic.Bool
}

func NewHealth(readiness Readiness, timeout time.Duration) *Health {
	return &Health{readiness: readiness, timeout: timeout}
}

func (health *Health) BeginDrain() {
	health.draining.Store(true)
}

func (health *Health) Ready(ctx context.Context) error {
	if health.draining.Load() {
		return errDraining
	}
	if health.readiness == nil {
		return errors.New("readiness checker is not configured")
	}
	probeContext, cancel := context.WithTimeout(ctx, health.timeout)
	defer cancel()
	return health.readiness.Ready(probeContext)
}
