package catalog_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/catalog/catalogtest"
)

// publish writes releases/<seq> under root and points current at it, as the catalog's publisher does.
func publish(t *testing.T, k *catalogtest.Kit, root string, seq int64, names ...string) string {
	t.Helper()
	var packs []catalogtest.Packed
	for _, n := range names {
		packs = append(packs, k.Pack(t, "harnsy/"+n, "1.0.0", sample(n), nil))
	}
	dir := filepath.Join(root, "releases", time.Unix(seq, 0).UTC().Format("20060102150405"))
	k.Release(t, dir, packs, func(f *catalog.Feed) { f.Seq = seq })
	tmp := filepath.Join(root, "current.tmp")
	os.Remove(tmp)
	if err := os.Symlink(dir, tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newStore(k *catalogtest.Kit, root string) (*catalog.Store, *time.Time) {
	now := k.Now
	return catalog.NewStore(root, k.Key, catalogtest.Host, func() time.Time { return now }), &now
}

func TestStoreFollowsCurrent(t *testing.T) {
	k, root := catalogtest.New(t), t.TempDir()
	s, _ := newStore(k, root)
	if err := s.Refresh(); err == nil || s.Release() != nil {
		t.Fatalf("no release yet: got err %v, release %v", err, s.Release())
	}
	publish(t, k, root, 1, "reviewer")
	if err := s.Refresh(); err != nil {
		t.Fatal(err)
	}
	if r := s.Release(); r == nil || r.Feed.Seq != 1 || len(r.Employees) != 1 {
		t.Fatalf("first release: %+v", r)
	}
	publish(t, k, root, 2, "reviewer", "tester")
	if err := s.Refresh(); err != nil {
		t.Fatal(err)
	}
	if r := s.Release(); r.Feed.Seq != 2 || r.Employee("harnsy/tester") == nil {
		t.Fatalf("second release: seq %d, %d employees", r.Feed.Seq, len(r.Employees))
	}
}

func TestStoreKeepsTheLastGoodRelease(t *testing.T) {
	k, root := catalogtest.New(t), t.TempDir()
	s, _ := newStore(k, root)
	publish(t, k, root, 1, "reviewer")
	if err := s.Refresh(); err != nil {
		t.Fatal(err)
	}
	bad := publish(t, k, root, 2, "reviewer", "tester")
	catalogtest.Write(t, filepath.Join(bad, "v1", "feed.json.sig"), []byte(`{"kid":"t1","sig":"AAAA"}`))
	if err := s.Refresh(); err == nil {
		t.Fatal("a release that does not verify must be reported")
	}
	if r := s.Release(); r == nil || r.Feed.Seq != 1 {
		t.Fatalf("want the last good release (seq 1), got %+v", r)
	}
	if err := s.Refresh(); err == nil {
		t.Fatal("the fault must be reported while current still points at the bad release")
	}
	publish(t, k, root, 3, "reviewer", "tester")
	if err := s.Refresh(); err != nil || s.Release().Feed.Seq != 3 {
		t.Fatalf("a later good release must be taken: err %v", err)
	}
}

func TestStoreStopsServingAnExpiredFeed(t *testing.T) {
	k, root := catalogtest.New(t), t.TempDir()
	s, now := newStore(k, root)
	publish(t, k, root, 1, "reviewer")
	if err := s.Refresh(); err != nil || s.Release() == nil {
		t.Fatalf("err %v", err)
	}
	*now = now.Add(8 * 24 * time.Hour) // the kit's feeds are good for a week
	if s.Release() != nil {
		t.Fatal("an expired feed must not be served")
	}
	if err := s.Refresh(); err == nil {
		t.Fatal("refresh must report the expired feed")
	}
}

// TestStoreServesAnOlderReleaseAndSaysSo: when current moves back to a release with a lower seq, the store serves it
// (a rollback by the publisher must keep working) and says that it is behind the highest seq it has held. Such a
// release may list a version revoked since, so the one who runs the server must hear of it.
func TestStoreServesAnOlderReleaseAndSaysSo(t *testing.T) {
	k, root := catalogtest.New(t), t.TempDir()
	s, _ := newStore(k, root)
	behind := func(wantSeq, wantHighest int64, want bool) {
		t.Helper()
		if err := s.Refresh(); err != nil {
			t.Fatal(err)
		}
		seq, highest, is := s.Behind()
		if r := s.Release(); r == nil || r.Feed.Seq != wantSeq || seq != wantSeq || highest != wantHighest || is != want {
			t.Fatalf("serving seq %d, highest %d, behind %v; want %d, %d, %v", seq, highest, is, wantSeq, wantHighest, want)
		}
	}
	if _, _, is := s.Behind(); is {
		t.Fatal("a store that holds nothing is not behind")
	}
	publish(t, k, root, 5, "reviewer")
	behind(5, 5, false)
	publish(t, k, root, 7, "reviewer", "tester")
	behind(7, 7, false)
	publish(t, k, root, 6, "reviewer")
	behind(6, 7, true)
	bad := publish(t, k, root, 9, "reviewer")
	catalogtest.Write(t, filepath.Join(bad, "v1", "feed.json.sig"), []byte(`{"kid":"t1","sig":"AAAA"}`))
	if err := s.Refresh(); err == nil {
		t.Fatal("a release that does not verify must be reported")
	}
	if seq, highest, is := s.Behind(); seq != 6 || highest != 7 || !is {
		t.Fatalf("a release that does not verify must not count: seq %d, highest %d, behind %v", seq, highest, is)
	}
	publish(t, k, root, 8, "reviewer")
	behind(8, 8, false)
}

// TestRunReportsEveryRelease: Run reports the first release it loads and each one after it, not only faults: the
// report is where a release that is older than the last is said aloud.
func TestRunReportsEveryRelease(t *testing.T) {
	k, root := catalogtest.New(t), t.TempDir()
	s, _ := newStore(k, root)
	publish(t, k, root, 2, "reviewer")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type report struct {
		err    error
		seq    int64
		behind bool
	}
	reports := make(chan report, 16)
	go s.Run(ctx, 5*time.Millisecond, func(err error) {
		seq, _, behind := s.Behind()
		reports <- report{err, seq, behind}
	})
	next := func(want report) {
		t.Helper()
		select {
		case got := <-reports:
			if (got.err == nil) != (want.err == nil) || got.seq != want.seq || got.behind != want.behind {
				t.Fatalf("report %+v, want %+v", got, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("no report, want %+v", want)
		}
	}
	next(report{nil, 2, false})
	publish(t, k, root, 3, "reviewer", "tester")
	next(report{nil, 3, false})
	publish(t, k, root, 1, "reviewer")
	next(report{nil, 1, true})
}
