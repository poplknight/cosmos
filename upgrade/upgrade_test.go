package upgrade

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// memStore is an in-memory Store for tests.
type memStore struct {
	mu      sync.Mutex
	version string
}

func (m *memStore) GetVersion(context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.version, nil
}

func (m *memStore) SetVersion(_ context.Context, v string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.version = v
	return nil
}

func TestVersionMapping(t *testing.T) {
	cases := []struct {
		str  string
		want int64
	}{
		{"v0.0.0", 0},
		{"v0.0.1", 1},
		{"v0.0.5", 5},
		{"v1.0.0", 1 << 32},
		{"v1.2.3", 1<<32 | 2<<16 | 3},
		{"v0.0.65535", 65535},
		{"v1.2.3", 1<<32 | 2<<16 | 3},
	}
	for _, c := range cases {
		got, err := parseVersion(c.str)
		if err != nil {
			t.Fatalf("parseVersion(%q): %v", c.str, err)
		}
		if got != c.want {
			t.Fatalf("parseVersion(%q) = %d, want %d", c.str, got, c.want)
		}
		// round trip: formatVersion(parseVersion(x)) == x
		if back := formatVersion(got); back != c.str {
			t.Fatalf("round trip failed: %q -> %d -> %q", c.str, got, back)
		}
	}
}

func TestParseVersionRejectsBadInput(t *testing.T) {
	bad := []string{"", "v", "v1", "v1.2", "v1.2.3.4", "v-1.2.3", "v1.2", "v1.2.65536", "v1..3", "va.b.c"}
	for _, s := range bad {
		if _, err := parseVersion(s); err == nil {
			t.Fatalf("parseVersion(%q) should fail", s)
		}
	}
}

func TestUpUpgradesInOrderWithJumps(t *testing.T) {
	store := &memStore{}
	var ran []string

	err := New(store).
		Register("v0.0.1", func(ctx context.Context) error { ran = append(ran, "v0.0.1"); return nil }).
		Register("v0.0.3", func(ctx context.Context) error { ran = append(ran, "v0.0.3"); return nil }).
		Register("v0.0.2", func(ctx context.Context) error { ran = append(ran, "v0.0.2"); return nil }).
		Up(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if len(ran) != 3 || ran[0] != "v0.0.1" || ran[1] != "v0.0.2" || ran[2] != "v0.0.3" {
		t.Fatalf("wrong execution order: %v", ran)
	}
	if got, _ := store.GetVersion(context.Background()); got != "v0.0.3" {
		t.Fatalf("final version = %q, want v0.0.3", got)
	}
}

func TestUpSkipsAlreadyReachedVersions(t *testing.T) {
	store := &memStore{version: "v0.0.2"}
	var ran []string

	err := New(store).
		Register("v0.0.1", func(ctx context.Context) error { ran = append(ran, "v0.0.1"); return nil }).
		Register("v0.0.2", func(ctx context.Context) error { ran = append(ran, "v0.0.2"); return nil }).
		Register("v0.0.3", func(ctx context.Context) error { ran = append(ran, "v0.0.3"); return nil }).
		Up(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || ran[0] != "v0.0.3" {
		t.Fatalf("should only run v0.0.3, got %v", ran)
	}
}

func TestUpStopsOnFailureAndResumes(t *testing.T) {
	store := &memStore{}
	attempts := 0
	ran1 := 0

	u := New(store).
		Register("v0.0.1", func(ctx context.Context) error { ran1++; return nil }).
		Register("v0.0.2", func(ctx context.Context) error {
			attempts++
			if attempts == 1 {
				return errors.New("boom")
			}
			return nil
		}).
		Register("v0.0.3", func(ctx context.Context) error { return nil })

	// first run: v0.0.2 fails, leaving the version at v0.0.1
	err := u.Up(context.Background())
	if err == nil || err.Error() != "upgrade to v0.0.2 failed: boom" {
		t.Fatalf("first run should stop at v0.0.2 with error, got %v", err)
	}
	if got, _ := store.GetVersion(context.Background()); got != "v0.0.1" {
		t.Fatalf("version should be v0.0.1 after failure, got %q", got)
	}

	// second run: resume from v0.0.2 without re-running v0.0.1
	if err := u.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ran1 != 1 {
		t.Fatalf("v0.0.1 should not run again, ran %d times", ran1)
	}
	if got, _ := store.GetVersion(context.Background()); got != "v0.0.3" {
		t.Fatalf("version should be v0.0.3 after resume, got %q", got)
	}
}

func TestUpTreatsEmptyVersionAsZero(t *testing.T) {
	store := &memStore{}
	ran := false
	if err := New(store).
		Register("v0.0.1", func(ctx context.Context) error { ran = true; return nil }).
		Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("empty current version should be treated as v0.0.0 and run upgrades")
	}
}

func TestRegisterRejectsDuplicateAndBadVersion(t *testing.T) {
	store := &memStore{}
	u := New(store).Register("v0.0.1", func(ctx context.Context) error { return nil })

	mustPanic := func(name string, fn func()) {
		defer func() {
			if recover() == nil {
				t.Fatalf("%s should panic", name)
			}
		}()
		fn()
	}
	mustPanic("duplicate registration", func() { u.Register("v0.0.1", func(ctx context.Context) error { return nil }) })
	mustPanic("malformed version", func() { u.Register("v1.2", func(ctx context.Context) error { return nil }) })
	mustPanic("nil function", func() { u.Register("v0.0.2", nil) })
}
