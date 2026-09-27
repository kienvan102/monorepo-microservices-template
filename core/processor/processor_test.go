package processor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestOutcome(t *testing.T) {
	boom := errors.New("boom")
	interrupted := fmt.Errorf("JS common/indexes: %w", context.Canceled)
	tests := []struct {
		name     string
		err      error
		sig      os.Signal
		wantCode int
		wantText string
	}{
		{"plain error", boom, nil, 1, "boom"},
		{"cancelled without a signal", interrupted, nil, 1, "context canceled"},
		{"plain error after a signal", boom, syscall.SIGTERM, 1, "boom"},
		{"interrupted by SIGTERM", interrupted, syscall.SIGTERM, 143, "stopped by SIGTERM before finishing"},
		{"interrupted by SIGINT", errors.Join(boom, interrupted), os.Interrupt, 130, "stopped by SIGINT before finishing"},
		{"exit error", &ExitError{Code: 3, Err: boom}, nil, 3, "boom"},
		{"wrapped exit error wins over a signal", fmt.Errorf("run: %w", &ExitError{Code: 4, Err: interrupted}), syscall.SIGTERM, 4, "context canceled"},
		{"exit error without cause", &ExitError{Code: 5}, nil, 5, "exit status 5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, code := outcome(tt.err, tt.sig)
			if code != tt.wantCode || !strings.Contains(text, tt.wantText) {
				t.Fatalf("outcome = (%q, %d), want code %d and text containing %q", text, code, tt.wantCode, tt.wantText)
			}
		})
	}
}

func TestNotifyStopReleaseWithoutSignal(t *testing.T) {
	ctx, received, release := notifyStop(context.Background())
	release()
	<-ctx.Done()
	if sig := received(); sig != nil {
		t.Fatalf("received() = %v after release without a signal, want nil", sig)
	}
}
