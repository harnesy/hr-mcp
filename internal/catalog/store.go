package catalog

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// Store holds the live release of a catalog root (<root>/current → releases/<seq>) and follows it. A release that does
// not verify is never taken: the last good one stays until it expires.
type Store struct {
	root string
	key  Key
	host string
	now  func() time.Time

	mu      sync.RWMutex
	rel     *Release
	relDir  string // the release directory rel was loaded from
	failDir string // the release directory that last failed, and why: not verified again until current moves
	failErr error
}

// NewStore makes a store for the catalog under root. It holds nothing until Refresh.
func NewStore(root string, key Key, host string, now func() time.Time) *Store {
	return &Store{root: root, key: key, host: host, now: now}
}

// Release returns the release to serve, or nil when there is none: nothing verified yet, or the feed expired.
func (s *Store) Release() *Release {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.rel == nil || !s.rel.Expires.After(s.now()) {
		return nil
	}
	return s.rel
}

// Refresh looks where current points and loads that release when it is not the one held. The error says why there is
// nothing new to serve: no release, one that does not verify (the last good one stays), or an expired feed.
func (s *Store) Refresh() error {
	dir, err := filepath.EvalSymlinks(filepath.Join(s.root, "current"))
	if err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	switch {
	case dir == s.relDir:
		if !s.rel.Expires.After(now) {
			return fmt.Errorf("catalog: the feed of %s expired at %s", dir, s.rel.Feed.Expires)
		}
		return nil
	case dir == s.failDir:
		return s.failErr
	}
	rel, err := Load(dir, s.key, s.host, now)
	if err != nil {
		s.failDir, s.failErr = dir, fmt.Errorf("catalog: %s refused: %w", dir, err)
		return s.failErr
	}
	s.rel, s.relDir, s.failDir, s.failErr = rel, dir, "", nil
	return nil
}

// Run refreshes every interval until ctx is done and reports each change of outcome: a fault once, not on every tick.
func (s *Store) Run(ctx context.Context, every time.Duration, report func(error)) {
	last := ""
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		msg := ""
		err := s.Refresh()
		if err != nil {
			msg = err.Error()
		}
		if msg != last {
			report(err)
			last = msg
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
