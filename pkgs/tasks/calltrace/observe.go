package calltrace

import (
	"context"
	"log/slog"
)

// RunObserved runs f with call-stack and helper.io logging: helper_in before, helper_out after.
// Use for helpers where you want explicit input/output key/value pairs in the JSON log (same
// alternating style as slog). The function name is pushed onto call_path for nested correlation.
func RunObserved(ctx context.Context, function string, inPairs []any, f func(context.Context) (outPairs []any, err error)) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = slog.Default().Enabled(ctx, slog.LevelDebug)
	ctx = Push(ctx, function)
	helperDebugIn(ctx, function, inPairs...)
	var outs []any
	defer func() {
		kv := append([]any{}, outs...)
		if err != nil {
			kv = append(kv, "err", err)
		}
		helperDebugOut(ctx, function, kv...)
	}()
	outs, err = f(ctx)
	return err
}

func helperDebugIn(ctx context.Context, fn string, kv ...any) {
	if ctx == nil || !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}
	args := []any{
		"cmd", LogCmd,
		"obs_category", "helper_io",
		"call_path", Path(ctx),
		"function", fn,
		"phase", "helper_in",
	}
	args = append(args, kv...)
	slog.Log(ctx, slog.LevelDebug, "helper.io", args...)
}

func helperDebugOut(ctx context.Context, fn string, kv ...any) {
	if ctx == nil || !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}
	args := []any{
		"cmd", LogCmd,
		"obs_category", "helper_io",
		"call_path", Path(ctx),
		"function", fn,
		"phase", "helper_out",
	}
	args = append(args, kv...)
	slog.Log(ctx, slog.LevelDebug, "helper.io", args...)
}

// HelperIOIn logs helper.io phase helper_in at Debug (handler JSON helpers and similar).
func HelperIOIn(ctx context.Context, fn string, kv ...any) {
	if ctx != nil {
		_ = slog.Default().Enabled(ctx, slog.LevelDebug)
	}
	helperDebugIn(ctx, fn, kv...)
}

// HelperIOOut logs helper.io phase helper_out at Debug.
func HelperIOOut(ctx context.Context, fn string, kv ...any) {
	if ctx != nil {
		_ = slog.Default().Enabled(ctx, slog.LevelDebug)
	}
	helperDebugOut(ctx, fn, kv...)
}
