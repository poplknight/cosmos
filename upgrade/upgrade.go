// Package upgrade provides versioned upgrades: register a list of "version → upgrade action"
// pairs, and Up steps from the current version to the latest, advancing only after each action
// succeeds.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Store reads and writes the current version. Implementations may be a file, a database, a
// remote service, or anything else. A program deployed at a customer service uses it to record
// how far upgrades have progressed, so Up can resume after a restart.
type Store interface {
	GetVersion(ctx context.Context) (string, error)
	SetVersion(ctx context.Context, version string) error
}

// Upgrade manages an ordered set of upgrade actions, from lower to higher versions.
type Upgrade struct {
	mu    sync.Mutex
	store Store
	steps map[int64]func(context.Context) error
}

// New returns an Upgrade that reads the current version from store and writes it back after
// each successful step.
func New(store Store) *Upgrade {
	return &Upgrade{store: store, steps: make(map[int64]func(context.Context) error)}
}

// Register records the action that upgrades the program to version and returns the receiver to
// allow chaining.
//
//	upgrade.New(store).
//		Register("v0.0.1", fn1).
//		Register("v0.0.2", fn2).
//		Up(ctx)
//
// version has the form vMAJOR.MINOR.PATCH with each segment in 0-65535. A malformed version or a
// duplicate registration panics: the list is static and should fail fast at initialization.
func (u *Upgrade) Register(version string, fn func(context.Context) error) *Upgrade {
	v, err := parseVersion(version)
	if err != nil {
		panic("upgrade: " + err.Error())
	}
	if fn == nil {
		panic(fmt.Sprintf("upgrade: upgrade function for version %s is nil", version))
	}
	if _, dup := u.steps[v]; dup {
		panic(fmt.Sprintf("upgrade: version %s already registered", version))
	}
	u.steps[v] = fn
	return u
}

// Up upgrades from the current version to the newest registered version.
//
// Each upgrade action advances the current version only after it succeeds; any failure stops the
// run and leaves the version where it was, so the next Up resumes from the breakpoint. Jumps are
// allowed: with v0.0.1 and v0.0.3 registered and the current version v0.0.0, Up runs v0.0.1 then
// v0.0.3. Registered versions ≤ the current one are skipped. Up is guarded by an internal lock, so
// concurrent calls are safe.
func (u *Upgrade) Up(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	cur, err := u.current(ctx)
	if err != nil {
		return err
	}

	versions := make([]int64, 0, len(u.steps))
	for v := range u.steps {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })

	for _, v := range versions {
		if v <= cur {
			continue
		}
		if err := u.steps[v](ctx); err != nil {
			return fmt.Errorf("upgrade to %s failed: %w", formatVersion(v), err)
		}
		if err := u.store.SetVersion(ctx, formatVersion(v)); err != nil {
			return fmt.Errorf("write version %s failed: %w", formatVersion(v), err)
		}
		cur = v
	}
	return nil
}

// CurrentVersion returns the current version string from the store; an empty (uninitialized)
// store is treated as v0.0.0.
func (u *Upgrade) CurrentVersion(ctx context.Context) (string, error) {
	v, err := u.current(ctx)
	if err != nil {
		return "", err
	}
	return formatVersion(v), nil
}

func (u *Upgrade) current(ctx context.Context) (int64, error) {
	s, err := u.store.GetVersion(ctx)
	if err != nil {
		return 0, err
	}
	if s == "" {
		return 0, nil // uninitialized store is treated as v0.0.0
	}
	v, err := parseVersion(s)
	if err != nil {
		return 0, fmt.Errorf("invalid current version %q: %w", s, err)
	}
	return v, nil
}

// parseVersion encodes vMAJOR.MINOR.PATCH into an int64: 16 bits per segment, following the
// IP dotted-decimal scheme, MAJOR<<32 | MINOR<<16 | PATCH.
func parseVersion(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return 0, errors.New("empty version")
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("version %q must have the form vMAJOR.MINOR.PATCH", s)
	}
	var seg [3]int64
	for i, p := range parts {
		x, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("segment %d of version %q is not a number", i+1, s)
		}
		if x < 0 || x > 0xFFFF {
			return 0, fmt.Errorf("segment %d of version %q exceeds 0-65535", i+1, s)
		}
		seg[i] = x
	}
	return seg[0]<<32 | seg[1]<<16 | seg[2], nil
}

// formatVersion is the inverse of parseVersion.
func formatVersion(v int64) string {
	return fmt.Sprintf("v%d.%d.%d", v>>32, (v>>16)&0xFFFF, v&0xFFFF)
}
