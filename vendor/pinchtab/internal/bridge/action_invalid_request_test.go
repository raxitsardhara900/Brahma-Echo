package bridge

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/config"
)

// A request naming no target is the caller having omitted a required argument, so every
// action that refuses one must say so with the sentinel — the endpoint answers 400 from that
// and from nothing else, and a plain fmt.Errorf lands in the catch-all that reports a typo as
// a server fault the caller should retry. The census is over the whole registry rather than
// the kinds this card measured: a new action refusing the same way is covered without a
// second edit.
func TestATargetlessActionIsRefusedWithTheInvalidRequestSentinel(t *testing.T) {
	b := New(context.TODO(), nil, &config.RuntimeConfig{})
	if len(b.Actions) == 0 {
		t.Fatal("the action registry is empty; this census would pass vacuously")
	}

	var refused []string
	for kind, action := range b.Actions {
		_, err := action(context.Background(), ActionRequest{})
		if err == nil || !strings.Contains(err.Error(), "need selector") {
			continue
		}
		refused = append(refused, kind)
		if !errors.Is(err, ErrInvalidActionRequest) {
			t.Errorf("%s refuses a targetless request with a bare error (%v); the endpoint cannot tell it from a browser fault and answers 500 retryable", kind, err)
		}
	}

	sort.Strings(refused)
	if len(refused) < 5 {
		t.Fatalf("only %v refused a targetless request; the census is no longer reaching the refusals it exists to check", refused)
	}
}
