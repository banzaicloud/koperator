package backoff

import "time"

// ConstantBackoffConfig holds the config for constant backoff policy
type ConstantBackoffConfig struct {
	Delay          time.Duration
	MaxElapsedTime time.Duration
	MaxRetries     int
}
