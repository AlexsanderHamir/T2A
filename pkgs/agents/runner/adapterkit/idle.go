package adapterkit

import (
	"context"
	"errors"
	"sync"
	"time"
)

// StreamIdleKind identifies a stdout-silence tier emitted before the run is
// cancelled for stuck recovery.
type StreamIdleKind int

const (
	StreamIdleSuspicious StreamIdleKind = iota
	StreamIdleKillPending
)

// ErrStreamIdle is stored as context.Cause when the idle watchdog kills a run.
var ErrStreamIdle = errors.New("adapterkit: stream idle")

// StreamIdleConfig configures stdout silence detection on streaming exec paths.
// Zero Stuck disables the watchdog entirely.
type StreamIdleConfig struct {
	Stuck  time.Duration
	OnIdle func(kind StreamIdleKind)
	Cancel context.CancelCauseFunc
}

// StreamIdleThresholds derives suspicious and kill-pending lead times from the
// stuck threshold. killLead defaults to 5s but shrinks when stuck is shorter.
func StreamIdleThresholds(stuck time.Duration) (suspicious, killPending time.Duration) {
	if stuck <= 0 {
		return 0, 0
	}
	suspicious = stuck / 2
	killLead := 5 * time.Second
	if stuck <= killLead {
		killLead = stuck / 2
		if killLead <= 0 {
			killLead = stuck
		}
	}
	killPending = stuck - killLead
	if killPending <= 0 || killPending <= suspicious {
		killPending = suspicious + (stuck-suspicious)/2
		if killPending >= stuck {
			killPending = stuck - time.Second
		}
		if killPending <= suspicious {
			killPending = suspicious + 1
		}
	}
	return suspicious, killPending
}

type streamIdleWatchdog struct {
	cfg          StreamIdleConfig
	suspicious   time.Duration
	killPending  time.Duration
	mu           sync.Mutex
	lastLineAt   time.Time
	seenLine     bool
	firedSusp    sync.Once
	firedKillMsg sync.Once
	firedStuck   sync.Once
	stop         chan struct{}
}

func newStreamIdleWatchdog(cfg StreamIdleConfig) *streamIdleWatchdog {
	suspicious, killPending := StreamIdleThresholds(cfg.Stuck)
	return &streamIdleWatchdog{
		cfg:         cfg,
		suspicious:  suspicious,
		killPending: killPending,
		stop:        make(chan struct{}),
	}
}

func (w *streamIdleWatchdog) wrap(onLine func([]byte)) func([]byte) {
	if w == nil || w.cfg.Stuck <= 0 {
		return onLine
	}
	return func(line []byte) {
		w.mu.Lock()
		w.lastLineAt = time.Now()
		w.seenLine = true
		w.mu.Unlock()
		if onLine != nil {
			onLine(line)
		}
	}
}

func (w *streamIdleWatchdog) run(ctx context.Context) {
	if w == nil || w.cfg.Stuck <= 0 {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.tick()
		}
	}
}

func (w *streamIdleWatchdog) tick() {
	w.mu.Lock()
	if !w.seenLine {
		w.mu.Unlock()
		return
	}
	idle := time.Since(w.lastLineAt)
	w.mu.Unlock()

	if idle >= w.suspicious {
		w.firedSusp.Do(func() {
			if w.cfg.OnIdle != nil {
				w.cfg.OnIdle(StreamIdleSuspicious)
			}
		})
	}
	if idle >= w.killPending {
		w.firedKillMsg.Do(func() {
			if w.cfg.OnIdle != nil {
				w.cfg.OnIdle(StreamIdleKillPending)
			}
		})
	}
	if idle >= w.cfg.Stuck {
		w.firedStuck.Do(func() {
			if w.cfg.Cancel != nil {
				w.cfg.Cancel(ErrStreamIdle)
			}
		})
	}
}

func (w *streamIdleWatchdog) close() {
	if w == nil {
		return
	}
	close(w.stop)
}
