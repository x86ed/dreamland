package zhougongdash

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"dreamland/internal/zhougongdata"
)

func ds(name string, runs int) zhougongdata.Dataset {
	d := zhougongdata.Dataset{Name: name, Branch: name, Source: "live"}
	for i := 0; i < runs; i++ {
		d.Runs = append(d.Runs, zhougongdata.Run{Agent: "morpheus", Commits: 1, Total: 100, Output: 50, LinesAdded: 10})
	}
	return d
}

func get(t *testing.T, h http.Handler, url string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
	b, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, string(b)
}

func TestCompareAPI_ThreeBranchDeltas(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Put(ds("A", 10))
	s.Put(ds("B", 15))
	s.Put(ds("C", 5))
	h := New(s).Handler()
	code, body := get(t, h, "/api/compare?branches=A,B,C&baseline=A")
	if code != 200 {
		t.Fatalf("code=%d %s", code, body)
	}
	var res zhougongdata.CompareResult
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatal(err)
	}
	row := res.Metrics[0]
	if *row.Cells[1].Delta != 5 || *row.Cells[1].DeltaPct != 50 || *row.Cells[2].Delta != -5 || *row.Cells[2].DeltaPct != -50 {
		t.Errorf("row=%+v", row)
	}
}

func TestCompareAPI_Limits(t *testing.T) {
	h := New(NewStore(t.TempDir())).Handler()
	for _, q := range []string{"", "A", "A,B,C,D,E,F,G,H,I"} {
		code, body := get(t, h, "/api/compare?branches="+q)
		if code != 400 || !strings.Contains(body, "between 2 and 8") {
			t.Errorf("branches=%q: code=%d body=%s", q, code, body)
		}
	}
	code, _ := get(t, h, "/api/compare?branches=A,B,C,D,E,F,G,H")
	if code != 200 {
		t.Errorf("8 branches should be accepted, got %d", code)
	}
}

func TestStore_ResolveAndArchivedMix(t *testing.T) {
	root := t.TempDir()
	old := ds("foo", 2)
	old.Branch = "12-foo"
	if _, err := zhougongdata.WriteArchive(root, "foo", old, time.Now()); err != nil {
		t.Fatal(err)
	}
	s := NewStore(root)
	s.Put(ds("bar", 3))
	s.Put(ds("bar", 4)) // replace
	s.Put(ds("foo", 1)) // collides with the archived slug

	all := s.All()
	if len(all) != 3 || all[2].Name != "foo (archived)" || all[2].Source != "archived" {
		t.Fatalf("all=%+v", names(all))
	}
	if len(s.Resolve("bar").Runs) != 4 {
		t.Errorf("replace failed")
	}
	if s.Resolve("12-fo").Source == "nodata" { // substring resolves branch/slug
		t.Errorf("substring should resolve")
	}
	if s.Resolve("nothing").Source != "nodata" {
		t.Errorf("unresolved should be nodata")
	}

	code, body := get(t, New(s).Handler(), "/api/compare?branches=bar,foo%20(archived),ghost")
	if code != 200 || !strings.Contains(body, `"archived"`) || !strings.Contains(body, `"noData":true`) {
		t.Errorf("mixed compare: %d %s", code, body)
	}
	code, body = get(t, New(s).Handler(), "/api/summary")
	if code != 200 || !strings.Contains(body, AttributionNote) || !strings.Contains(body, "openspec/**") {
		t.Errorf("summary: %d %s", code, body)
	}
	code, body = get(t, New(s).Handler(), "/")
	if code != 200 || !strings.Contains(body, "ZHOUGONG//METRICS") {
		t.Errorf("index: %d", code)
	}
}

func names(all []zhougongdata.Dataset) []string {
	var n []string
	for _, d := range all {
		n = append(n, d.Name)
	}
	return n
}

func TestDashboard_LoopbackAndStopReleasesPort(t *testing.T) {
	d := New(NewStore(t.TempDir()))
	if d.Stop() != nil || d.Addr() != "" {
		t.Fatal("stop on stopped dashboard must be a no-op")
	}
	url, err := d.Start(0)
	if err != nil {
		t.Fatal(err)
	}
	url2, _ := d.Start(0)
	if url != url2 {
		t.Errorf("start must be idempotent: %s vs %s", url, url2)
	}
	host, _, err := net.SplitHostPort(d.Addr())
	if err != nil || host != "127.0.0.1" {
		t.Fatalf("addr=%s", d.Addr())
	}
	resp, err := http.Get(url + "/api/summary")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if err := d.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := http.Get(url + "/api/summary"); err == nil {
		t.Errorf("request after stop should fail to connect")
	}
	if d.Stop() != nil {
		t.Errorf("second stop")
	}
}

func TestDashboard_StartFailsWhenPortBusy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if _, err := New(NewStore(t.TempDir())).Start(ln.Addr().(*net.TCPAddr).Port); err == nil {
		t.Errorf("expected bind error")
	}
}

func TestSummary_CurrentBranch(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"}, {"checkout", "-q", "-b", "feat-x"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("git unavailable: %v %s", err, out)
		}
	}
	s := NewStore(root)
	s.Put(ds("feat-x", 2))
	s.Put(ds("other", 1))
	_, body := get(t, New(s).Handler(), "/api/summary")
	var res struct {
		CurrentBranch  string `json:"currentBranch"`
		CurrentDataset string `json:"currentDataset"`
	}
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatal(err)
	}
	if res.CurrentBranch != "feat-x" || res.CurrentDataset != "feat-x" {
		t.Errorf("got %+v", res)
	}

	_, body = get(t, New(NewStore(t.TempDir())).Handler(), "/api/summary")
	if !strings.Contains(body, `"currentBranch":""`) {
		t.Errorf("non-repo should give empty currentBranch: %s", body)
	}
}

func TestSummary_CollectsCurrentBranchOnce(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"}, {"checkout", "-q", "-b", "feat-y"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("git unavailable: %v %s", err, out)
		}
	}
	h := New(NewStore(root)).Handler()
	_, body := get(t, h, "/api/summary")
	var res struct {
		CurrentDataset string `json:"currentDataset"`
		CollectError   string `json:"collectError"`
	}
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatal(err)
	}
	if res.CurrentDataset != "feat-y" || res.CollectError != "" {
		t.Fatalf("got %+v", res)
	}
	e, ok, err := zhougongdata.Read(root, "feat-y")
	if err != nil || !ok || e.HeadSha == "" {
		t.Fatalf("cache not written: %v %v %+v", err, ok, e)
	}
	e.Dataset.Runs = append(e.Dataset.Runs, zhougongdata.Run{Agent: "sentinel"})
	if err := zhougongdata.Write(root, e); err != nil {
		t.Fatal(err)
	}
	_, body = get(t, h, "/api/summary")
	if !strings.Contains(body, "sentinel") {
		t.Errorf("second call re-parsed instead of reusing cache")
	}
}

func TestSummary_CollectError(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q", "-b", "empty").CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	_, body := get(t, New(NewStore(root)).Handler(), "/api/summary")
	if !strings.Contains(body, `"collectError"`) || strings.Contains(body, `"collectError":""`) {
		t.Errorf("expected collectError for branch with no commits: %s", body)
	}
}
