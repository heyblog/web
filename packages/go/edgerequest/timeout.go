package edgerequest

import (
	"context"
	"time"
)

// networkBudget is shared by all DNS lookups and redirect hops in one Execute.
// Task authorization and pacing never reset or consume this network allowance.
type networkBudget struct{ remaining time.Duration }

func (b *networkBudget) start(ctx context.Context) (context.Context, func()) {
	started := time.Now()
	network, cancel := context.WithTimeout(ctx, b.remaining)
	return network, func() {
		b.remaining -= time.Since(started)
		cancel()
	}
}
