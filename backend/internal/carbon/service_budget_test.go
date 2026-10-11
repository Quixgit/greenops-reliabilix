package carbon

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/apiusage"
)

func TestARetryThatCannotSucceedIsNotRetried(t *testing.T) {
	for _, e := range []error{apiusage.ErrBudget, domain.ErrNotConfigured, domain.ErrPastUnavailable, fmt.Errorf("x: %w", apiusage.ErrBudget)} {
		if err := noRetryOnBudget(e); !errors.Is(err, asynq.SkipRetry) || !errors.Is(err, e) {
			t.Errorf("%v must not be retried: %v", e, err)
		}
	}
	other := errors.New("provider returned 502")
	if err := noRetryOnBudget(other); errors.Is(err, asynq.SkipRetry) || err != other {
		t.Errorf("a transient failure must still be retried: %v", err)
	}
	if noRetryOnBudget(nil) != nil {
		t.Error("nil stays nil")
	}
}
