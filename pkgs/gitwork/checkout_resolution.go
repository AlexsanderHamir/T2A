package gitwork

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const maxDiscoverSiblingDirs = 32

// Resolution source labels returned in ResolveResult.Source.
const (
	ResolveSourceCache      = "cache"
	ResolveSourceCandidate  = "candidate"
	ResolveSourceDiscovered = "discovered"
)

var (
	// ErrBootstrapMismatch is returned when a candidate checkout is not the same repository.
	ErrBootstrapMismatch = errors.New("gitwork: bootstrap path is not the same repository")
	// ErrAmbiguousDiscovery is returned when more than one sibling checkout matches identity.
	ErrAmbiguousDiscovery = errors.New("gitwork: ambiguous checkout discovery")
)

// RegisteredCheckout holds DB cache fields needed to resolve and verify a repository.
type RegisteredCheckout struct {
	CachedMainPath  string
	CachedCommonDir string
	BranchHeads     map[string]string // branch name → head SHA; may be empty values
}

// ResolveInput configures OpenRegisteredCheckout.
type ResolveInput struct {
	Registered    RegisteredCheckout
	CandidatePath string // operator bootstrap; optional
	AllowDiscover bool   // sibling scan under parent of CachedMainPath
}

// ResolveResult is a successfully opened repository checkout.
type ResolveResult struct {
	Repo           *Repository
	OpenedPath     string
	Source         string
	DiscoveredFrom string // set when Source == ResolveSourceDiscovered
}

// OpenRegisteredCheckout tries cached path, then candidate, then optional sibling discovery.
// When no checkout opens, returns zero ResolveResult and nil error.
func (s *DefaultService) OpenRegisteredCheckout(ctx context.Context, input ResolveInput) (ResolveResult, error) {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.OpenRegisteredCheckout")
	if opened, err := tryOpenCheckoutPath(ctx, s, input.Registered.CachedMainPath); err != nil {
		if !isCheckoutPathMissing(err, input.Registered.CachedMainPath) {
			return ResolveResult{}, fmt.Errorf("open cached repository: %w", err)
		}
	} else {
		return ResolveResult{
			Repo:       opened,
			OpenedPath: opened.Root,
			Source:     ResolveSourceCache,
		}, nil
	}

	candidate := strings.TrimSpace(input.CandidatePath)
	if candidate != "" {
		opened, err := s.OpenRepository(ctx, candidate)
		if err != nil {
			if errors.Is(err, ErrNotARepository) {
				return ResolveResult{}, ErrNotARepository
			}
			return ResolveResult{}, fmt.Errorf("open candidate repository: %w", err)
		}
		if err := s.VerifySameRepository(ctx, input.Registered, opened); err != nil {
			return ResolveResult{}, err
		}
		return ResolveResult{
			Repo:       opened,
			OpenedPath: opened.Root,
			Source:     ResolveSourceCandidate,
		}, nil
	}

	if !input.AllowDiscover {
		return ResolveResult{}, nil
	}
	discovered, err := s.DiscoverCheckoutNearby(ctx, input.Registered)
	if err != nil {
		return ResolveResult{}, err
	}
	if discovered == nil {
		return ResolveResult{}, nil
	}
	return ResolveResult{
		Repo:           discovered,
		OpenedPath:     discovered.Root,
		Source:         ResolveSourceDiscovered,
		DiscoveredFrom: discovered.Root,
	}, nil
}

// VerifySameRepository checks that candidate shares the registered object database.
func (s *DefaultService) VerifySameRepository(ctx context.Context, registered RegisteredCheckout, candidate *Repository) error {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.VerifySameRepository")
	if candidate == nil {
		return ErrBootstrapMismatch
	}
	if len(registered.BranchHeads) == 0 {
		cd := strings.TrimSpace(registered.CachedCommonDir)
		if cd != "" && PathKeyEqual(candidate.CommonDir, cd) {
			return nil
		}
		return ErrBootstrapMismatch
	}
	for name, storedSHA := range registered.BranchHeads {
		branch := strings.TrimSpace(name)
		storedSHA = strings.TrimSpace(storedSHA)
		if branch == "" || storedSHA == "" {
			continue
		}
		liveHead, err := s.BranchHead(ctx, candidate, branch)
		if err != nil {
			continue
		}
		if strings.EqualFold(storedSHA, strings.TrimSpace(liveHead)) {
			return nil
		}
	}
	if cd := strings.TrimSpace(registered.CachedCommonDir); cd != "" {
		if PathKeyEqual(candidate.CommonDir, cd) {
			return nil
		}
	}
	return ErrBootstrapMismatch
}

// DiscoverCheckoutNearby scans sibling directories under the parent of CachedMainPath.
// Returns nil when no match. When multiple siblings verify, prefers the main checkout.
func (s *DefaultService) DiscoverCheckoutNearby(ctx context.Context, registered RegisteredCheckout) (*Repository, error) {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.DiscoverCheckoutNearby")
	parent := filepath.Dir(strings.TrimSpace(registered.CachedMainPath))
	if parent == "" || parent == registered.CachedMainPath {
		return nil, nil
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read parent directory: %w", err)
	}
	var matches []*Repository
	seen := 0
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		seen++
		if seen > maxDiscoverSiblingDirs {
			break
		}
		candidatePath := filepath.Join(parent, ent.Name())
		opened, err := s.OpenRepository(ctx, candidatePath)
		if err != nil {
			if errors.Is(err, ErrNotARepository) {
				continue
			}
			return nil, fmt.Errorf("open sibling %q: %w", candidatePath, err)
		}
		if err := s.VerifySameRepository(ctx, registered, opened); err != nil {
			if errors.Is(err, ErrBootstrapMismatch) {
				continue
			}
			return nil, err
		}
		matches = append(matches, opened)
	}
	return s.pickMainAmongMatches(ctx, matches)
}

func (s *DefaultService) pickMainAmongMatches(ctx context.Context, matches []*Repository) (*Repository, error) {
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return matches[0], nil
	}
	var mainCandidates []*Repository
	for _, m := range matches {
		mainRoot, _, err := s.ResolveRegistration(ctx, m.Root)
		if err != nil {
			continue
		}
		if PathKeyEqual(m.Root, mainRoot) {
			mainCandidates = append(mainCandidates, m)
		}
	}
	if len(mainCandidates) == 1 {
		return mainCandidates[0], nil
	}
	return nil, ErrAmbiguousDiscovery
}

//funclogmeasure:skip category=hot-path reason="Open helper; operation trace is emitted by OpenRegisteredCheckout."
func tryOpenCheckoutPath(ctx context.Context, svc Service, path string) (*Repository, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, ErrNotARepository
	}
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		if err != nil && os.IsNotExist(err) {
			return nil, ErrNotARepository
		}
		if err != nil {
			return nil, err
		}
		return nil, ErrNotARepository
	}
	return svc.OpenRepository(ctx, path)
}

//funclogmeasure:skip category=hot-path reason="Pure helper; operation trace is emitted by OpenRegisteredCheckout."
func isCheckoutPathMissing(err error, path string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotARepository) {
		return true
	}
	if _, statErr := os.Stat(strings.TrimSpace(path)); os.IsNotExist(statErr) {
		return true
	}
	return false
}
