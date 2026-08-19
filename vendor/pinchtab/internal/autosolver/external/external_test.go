package external

import (
	"context"
	"testing"

	"github.com/pinchtab/pinchtab/internal/autosolver"
)

// The reported solver identity has to be the registered one. Solve now builds it
// from Name(), which makes drift impossible rather than merely detectable; this
// test reds if either one goes back to spelling the name a second time.
func TestSolverUsedMatchesTheRegisteredName(t *testing.T) {
	for _, solver := range []autosolver.Solver{
		NewCapsolver(CapsolverConfig{}),
		NewTwoCaptcha(TwoCaptchaConfig{}),
	} {
		t.Run(solver.Name(), func(t *testing.T) {
			// An unset API key is the earliest return, and it is enough: the
			// Result is built before the check.
			result, err := solver.Solve(context.Background(), nil, nil)
			if err == nil {
				t.Fatal("expected the unset-API-key error, so this test exercises the real Solve path")
			}
			if result == nil {
				t.Fatal("Solve returned no Result to check")
			}
			if result.SolverUsed != solver.Name() {
				t.Errorf("Result.SolverUsed = %q but Name() = %q — the reported solver identity has drifted from the registered one", result.SolverUsed, solver.Name())
			}
		})
	}
}
