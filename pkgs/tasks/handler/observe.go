package handler

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
	ctx = PushCall(ctx, function)
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
