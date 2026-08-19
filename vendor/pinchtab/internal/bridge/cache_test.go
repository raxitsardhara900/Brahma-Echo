package bridge

import (
	"context"
	"testing"
)

func TestClearCache_RequiresContext(t *testing.T) {
	b := &Bridge{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	err := b.ClearCache(ctx)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

func TestCanClearCache_RequiresContext(t *testing.T) {
	b := &Bridge{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	_, err := b.CanClearCache(ctx)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}
