package backoff

import (
	"context"

	"github.com/goph/emperror"
	"github.com/lestrrat-go/backoff"
)

// NewConstantBackoffPolicy creates a new constant backoff policy
func NewConstantBackoffPolicy(config *ConstantBackoffConfig) *backoff.Constant {
	return backoff.NewConstant(config.Delay, backoff.WithMaxRetries(config.MaxRetries), backoff.WithMaxElapsedTime(config.MaxElapsedTime))
}

// Retry retries the given function using a backoff policy
func Retry(function func() error, backoffPolicy backoff.Policy) (err error) {
	b, cancel := backoffPolicy.Start(context.Background())

	defer cancel()
	for {
		select {
		case <-b.Done():
			return emperror.Wrap(err, "all attempts failed")
		case <-b.Next():
			err = function()
			if err == nil {
				return nil
			}
			if backoff.IsPermanentError(err) {
				return emperror.Wrap(err, "permanent error happened during retrying")
			}
		}
	}
}

// MarkErrorPermanent marks an error permanent error so it won't be retried (unlike with non-marked errors considered as transient)
func MarkErrorPermanent(err error) error {
	return backoff.MarkPermanent(err)
}
