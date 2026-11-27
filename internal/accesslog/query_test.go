package accesslog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func recordSample(l *Logger) {
	now := time.Now()
	for i := 0; i < 8; i++ {
		l.Record(Entry{
			Time:        now.Add(time.Duration(i) * time.Second),
			ClientIP:    "203.0.113." + string(rune('1'+i)),
			Domain:      "example.com",
			Status:      200,
			BlockReason: "",
		})
	}
	for i := 0; i < 3; i++ {
		l.Record(Entry{
			Time:        now.Add(time.Duration(10+i) * time.Second),
			ClientIP:    "198.51.100." + string(rune('1'+i)),
			Domain:      "example.com",
			Status:      403,
			Blocked:     true,
			BlockReason: "crowdsec",
		})
	}
}

func TestQueryPaginationAndFilters(t *testing.T) {
	l := New(100)
	recordSample(l)

	blocked := true
	entries, total := l.Query(Query{Blocked: &blocked, Limit: 2, Offset: 0})
	if total != 3 || len(entries) != 2 {
		t.Fatalf("total=%d len=%d", total, len(entries))
	}
	// Newest first.
	if entries[0].ClientIP != "198.51.100.3" || entries[1].ClientIP != "198.51.100.2" {
		t.Fatalf("unexpected order: %+v", entries)
	}

	entries, total = l.Query(Query{Blocked: &blocked, Limit: 2, Offset: 2})
	if total != 3 || len(entries) != 1 || entries[0].ClientIP != "198.51.100.1" {
		t.Fatalf("page 2: total=%d entries=%+v", total, entries)
	}

	entries, total = l.Query(Query{Reason: "crowd", Limit: 10})
	if total != 3 || len(entries) != 3 {
		t.Fatalf("reason filter: total=%d len=%d", total, len(entries))
	}

	entries, total = l.Query(Query{Status: 200, Limit: 10})
	if total != 8 || len(entries) != 8 {
		t.Fatalf("status filter: total=%d len=%d", total, len(entries))
	}

	entries, total = l.Query(Query{Status: 403, Limit: 10, Offset: 99})
	if total != 3 || len(entries) != 0 {
		t.Fatalf("offset beyond: total=%d len=%d", total, len(entries))
	}
}

func TestQueryFilePagination(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	var lines []byte
	for i := 0; i < 5; i++ {
		e := Entry{
			Time:        now.Add(time.Duration(i) * time.Second),
			ClientIP:    "192.0.2." + string(rune('1'+i)),
			Domain:      "example.com",
			Status:      403,
			Blocked:     true,
			BlockReason: "crowdsec",
		}
		b, _ := json.Marshal(e)
		lines = append(lines, b...)
		lines = append(lines, '\n')
	}
	name := "access-2026-11-27.jsonl"
	if err := os.WriteFile(filepath.Join(dir, name), lines, 0o640); err != nil {
		t.Fatal(err)
	}

	l := &Logger{dir: dir}
	entries, total := l.QueryFile(name, Query{Limit: 2, Offset: 0})
	if total != 5 || len(entries) != 2 {
		t.Fatalf("total=%d len=%d", total, len(entries))
	}
	if entries[0].ClientIP != "192.0.2.5" || entries[1].ClientIP != "192.0.2.4" {
		t.Fatalf("unexpected order: %+v", entries)
	}
	entries, total = l.QueryFile(name, Query{Limit: 2, Offset: 4})
	if total != 5 || len(entries) != 1 || entries[0].ClientIP != "192.0.2.1" {
		t.Fatalf("last page: total=%d entries=%+v", total, entries)
	}
	entries, total = l.QueryFile(name, Query{Limit: 10, Reason: "nope"})
	if total != 0 || len(entries) != 0 {
		t.Fatalf("reason miss: total=%d len=%d", total, len(entries))
	}
}
