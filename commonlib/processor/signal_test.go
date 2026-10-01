//go:build unix

package processor

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestNotifyStopRecordsSignal(t *testing.T) {
	ctx, received, release := notifyStop(context.Background())
	defer release()
	if sig := received(); sig != nil {
		t.Fatalf("received() = %v before any signal, want nil", sig)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("context was not cancelled by SIGTERM")
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("ctx.Err() = %v, want context.Canceled", ctx.Err())
	}
	if sig := received(); sig != syscall.SIGTERM {
		t.Fatalf("received() = %v, want SIGTERM", sig)
	}
}
