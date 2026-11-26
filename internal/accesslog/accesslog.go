package accesslog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Time        time.Time `json:"time"`
	ClientIP    string    `json:"client_ip"`
	Host        string    `json:"host"`
	Domain      string    `json:"domain"`
	SiteID      string    `json:"site_id"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	Status      int       `json:"status"`
	Bytes       int64     `json:"bytes"`
	DurationMs  int64     `json:"duration_ms"`
	Blocked     bool      `json:"blocked"`
	BlockReason string    `json:"block_reason"`
	Country     string    `json:"country"`
	UserAgent   string    `json:"user_agent"`
	Referer     string    `json:"referer"`
	Proto       string    `json:"proto"`
	Admin       bool      `json:"admin"`
}

type SiteStat struct {
	Domain  string `json:"domain"`
	Total   int64  `json:"total"`
	Blocked int64  `json:"blocked"`
}

type CountryStat struct {
	Country string `json:"country"`
	Count   int64  `json:"count"`
}

type Stats struct {
	UptimeSeconds    int64            `json:"uptime_seconds"`
	Total            int64            `json:"total"`
	Blocked          int64            `json:"blocked"`
	Allowed          int64            `json:"allowed"`
	ByStatus         map[string]int64 `json:"by_status"`
	ByReason         map[string]int64 `json:"by_reason"`
	TopSites         []SiteStat       `json:"top_sites"`
	BlockedCountries []CountryStat    `json:"blocked_countries"`
	RPS              float64          `json:"rps"`
}

type Query struct {
	Site    string
	Blocked *bool
	Status  int
	Search  string
	Limit        int
	ExcludeAdmin bool
	Domains map[string]bool
}

type LogFile struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type Logger struct {
	mu      sync.RWMutex
	cap     int
	entries []Entry
	head    int
	count   int

	started    time.Time
	statsSince time.Time

	total            int64
	blocked          int64
	byStatus         map[int]int64
	byReason         map[string]int64
	sites            map[string]*SiteStat
	blockedCountries map[string]int64

	windowCachedAt time.Time
	windowStats    Stats

	dir     string
	writeCh chan Entry
	stopCh  chan struct{}
	doneCh  chan struct{}

	curDate string
	file    *os.File
	writer  *bufio.Writer

	combined  *os.File
	combinedW *bufio.Writer
}

func New(capacity int) *Logger {
	return newLogger("", capacity)
}

func NewWithPersistence(dir string, capacity, retentionDays int) *Logger {
	l := newLogger(dir, capacity)
	if dir == "" {
		return l
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Printf("accesslog: mkdir %s: %v", dir, err)
		l.dir = ""
		return l
	}
	l.cleanupOld(retentionDays)
	l.loadRecent()
	if f, err := os.OpenFile(filepath.Join(dir, "access.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640); err == nil {
		l.combined = f
		l.combinedW = bufio.NewWriterSize(f, 64*1024)
	}
	l.writeCh = make(chan Entry, 8192)
	l.stopCh = make(chan struct{})
	l.doneCh = make(chan struct{})
	go l.writerLoop()
	return l
}

func newLogger(dir string, capacity int) *Logger {
	if capacity <= 0 {
		capacity = 10000
	}
	now := time.Now()
	return &Logger{
		cap:              capacity,
		entries:          make([]Entry, capacity),
		started:          now,
		statsSince:       now,
		byStatus:         map[int]int64{},
		byReason:         map[string]int64{},
		sites:            map[string]*SiteStat{},
		blockedCountries: map[string]int64{},
		dir:              dir,
	}
}

func (l *Logger) Record(e Entry) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	l.mu.Lock()
	l.entries[l.head] = e
	l.head = (l.head + 1) % l.cap
	if l.count < l.cap {
		l.count++
	}
	if !e.Admin {
		if !e.Admin {
			l.total++
			l.byStatus[e.Status]++
			if e.Blocked {
				l.blocked++
				l.byReason[e.BlockReason]++
				if e.Country != "" {
					l.blockedCountries[e.Country]++
				}
			}
			if e.Domain != "" {
				ss := l.sites[e.Domain]
				if ss == nil {
					ss = &SiteStat{Domain: e.Domain}
					l.sites[e.Domain] = ss
				}
				ss.Total++
				if e.Blocked {
					ss.Blocked++
				}
			}
		}
	}
	l.mu.Unlock()

	if l.writeCh != nil {
		select {
		case l.writeCh <- e:
		default:
		}
	}
}

func (l *Logger) Close() {
	if l.writeCh == nil {
		return
	}
	close(l.stopCh)
	<-l.doneCh
	if l.combinedW != nil {
		_ = l.combinedW.Flush()
	}
	if l.combined != nil {
		_ = l.combined.Close()
	}
}

func (l *Logger) writerLoop() {
	defer close(l.doneCh)
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	batch := make([]Entry, 0, 256)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		l.writeBatch(batch)
		batch = batch[:0]
	}

	for {
		select {
		case e := <-l.writeCh:
			batch = append(batch, e)
			if len(batch) >= 256 {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-l.stopCh:
			for {
				select {
				case e := <-l.writeCh:
					batch = append(batch, e)
				default:
					flush()
					l.closeFile()
					return
				}
			}
		}
	}
}

func (l *Logger) writeCombined(e Entry) {
	proto := e.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	ref := e.Referer
	if ref == "" {
		ref = "-"
	}
	ua := e.UserAgent
	if ua == "" {
		ua = "-"
	}
	path := e.Path
	if path == "" {
		path = "/"
	}
	fmt.Fprintf(l.combinedW, "%s - - [%s] \"%s %s %s\" %d %d \"%s\" \"%s\"\n",
		e.ClientIP, e.Time.Format("02/Jan/2006:15:04:05 -0700"), e.Method, path, proto, e.Status, e.Bytes, ref, ua)
}

func (l *Logger) writeBatch(batch []Entry) {
	today := time.Now().Format("2006-01-02")
	if l.curDate != today || l.file == nil {
		l.closeFile()
		if err := l.openFile(today); err != nil {
			log.Printf("accesslog: open file: %v", err)
			return
		}
	}
	for _, e := range batch {
		b, err := json.Marshal(e)
		if err != nil {
			continue
		}
		if _, err := l.writer.Write(b); err != nil {
			return
		}
		if err := l.writer.WriteByte('\n'); err != nil {
			return
		}
	}
	_ = l.writer.Flush()

	if l.combinedW != nil {
		for _, e := range batch {
			l.writeCombined(e)
		}
		_ = l.combinedW.Flush()
	}
}

func (l *Logger) openFile(date string) error {
	path := filepath.Join(l.dir, "access-"+date+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return err
	}
	l.file = f
	l.writer = bufio.NewWriterSize(f, 64*1024)
	l.curDate = date
	return nil
}

func (l *Logger) closeFile() {
	if l.writer != nil {
		_ = l.writer.Flush()
	}
	if l.file != nil {
		_ = l.file.Close()
	}
	l.file = nil
	l.writer = nil
	l.curDate = ""
}

func (l *Logger) cleanupOld(days int) {
	if days <= 0 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	files, _ := l.Files()
	for _, f := range files {
		if f.Modified.Before(cutoff) {
			_ = os.Remove(filepath.Join(l.dir, f.Name))
		}
	}
}

func (l *Logger) loadRecent() {
	files, _ := l.Files()
	if len(files) == 0 {
		return
	}
	path := filepath.Join(l.dir, files[len(files)-1].Name)
	entries := readTail(path, l.cap)
	l.mu.Lock()
	for _, e := range entries {
		l.entries[l.head] = e
		l.head = (l.head + 1) % l.cap
		if l.count < l.cap {
			l.count++
		}
	}
	l.mu.Unlock()
	l.recomputeStats()
}

// recomputeStats rebuilds all counters from the in-memory ring buffer so the
// dashboard continues from the persisted logs after a restart.
func (l *Logger) recomputeStats() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.total = 0
	l.blocked = 0
	l.byStatus = map[int]int64{}
	l.byReason = map[string]int64{}
	l.sites = map[string]*SiteStat{}
	l.blockedCountries = map[string]int64{}

	var oldest time.Time
	for i := 0; i < l.count; i++ {
		idx := (l.head - l.count + i + l.cap*2) % l.cap
		e := l.entries[idx]
		if e.Time.IsZero() {
			continue
		}
		if oldest.IsZero() || e.Time.Before(oldest) {
			oldest = e.Time
		}
		if !e.Admin {
			l.total++
			l.byStatus[e.Status]++
			if e.Blocked {
				l.blocked++
				l.byReason[e.BlockReason]++
				if e.Country != "" {
					l.blockedCountries[e.Country]++
				}
			}
			if e.Domain != "" {
				ss := l.sites[e.Domain]
				if ss == nil {
					ss = &SiteStat{Domain: e.Domain}
					l.sites[e.Domain] = ss
				}
				ss.Total++
				if e.Blocked {
					ss.Blocked++
				}
			}
		}
	}
	if !oldest.IsZero() {
		l.statsSince = oldest
	}
}

func (l *Logger) Files() ([]LogFile, error) {
	if l.dir == "" {
		return nil, nil
	}
	ents, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, err
	}
	var out []LogFile
	for _, e := range ents {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "access-") || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, LogFile{Name: e.Name(), Size: info.Size(), Modified: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (l *Logger) QueryFile(name string, q Query) []Entry {
	if l.dir == "" || name == "" {
		return nil
	}
	if strings.ContainsAny(name, "/\\") || !strings.HasPrefix(name, "access-") || !strings.HasSuffix(name, ".jsonl") {
		return nil
	}
	limit := q.Limit
	if limit <= 0 || limit > 5000 {
		limit = 300
	}
	entries := readTail(filepath.Join(l.dir, name), 5000)
	out := make([]Entry, 0, limit)
	for i := len(entries) - 1; i >= 0; i-- {
		if !match(entries[i], q) {
			continue
		}
		out = append(out, entries[i])
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (l *Logger) Query(q Query) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	out := make([]Entry, 0, limit)
	for i := 0; i < l.count; i++ {
		idx := (l.head - 1 - i + l.cap*2) % l.cap
		e := l.entries[idx]
		if !match(e, q) {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func match(e Entry, q Query) bool {
	if q.ExcludeAdmin && e.Admin {
		return false
	}
	if q.Domains != nil && !q.Domains[e.Domain] {
		return false
	}
	if q.Site != "" && !strings.EqualFold(e.Domain, q.Site) && !strings.EqualFold(e.Host, q.Site) {
		return false
	}
	if q.Blocked != nil && e.Blocked != *q.Blocked {
		return false
	}
	if q.Status != 0 && e.Status != q.Status {
		return false
	}
	if q.Search != "" {
		s := strings.ToLower(q.Search)
		if !strings.Contains(strings.ToLower(e.Path), s) &&
			!strings.Contains(strings.ToLower(e.ClientIP), s) &&
			!strings.Contains(strings.ToLower(e.Host), s) {
			return false
		}
	}
	return true
}

// SiteRangeStats is the per-site aggregation used by the site statistics page.
type SiteRangeStats struct {
	Total    int64            `json:"total"`
	Blocked  int64            `json:"blocked"`
	Allowed  int64            `json:"allowed"`
	ByStatus map[string]int64 `json:"by_status"`
	ByReason map[string]int64 `json:"by_reason"`
}

// normalizeHost lowercases a Host header and strips an optional port.
func normalizeHost(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if strings.HasPrefix(h, "[") {
		if i := strings.LastIndex(h, "]"); i > 0 {
			return h[1:i]
		}
	}
	if i := strings.LastIndexByte(h, ':'); i >= 0 && strings.Count(h, ":") == 1 {
		return h[:i]
	}
	return h
}

// SiteStatsSince aggregates log entries per canonical site domain since the
// given time. hostToDomain maps every hostname (primary domain and alias
// domains, lowercase) to the site's primary domain. Entries that were blocked
// before the site lookup (blacklist, CrowdSec) carry no Domain and are
// attributed via their Host header. A zero since means "everything retained".
func (l *Logger) SiteStatsSince(since time.Time, hostToDomain map[string]string) map[string]SiteRangeStats {
	out := map[string]SiteRangeStats{}
	l.mu.Lock()
	dir := l.dir
	l.mu.Unlock()
	if dir == "" || len(hostToDomain) == 0 {
		return out
	}
	files, _ := l.Files()
	for _, f := range files {
		fh, err := os.Open(filepath.Join(dir, f.Name))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			var e Entry
			if json.Unmarshal(sc.Bytes(), &e) != nil {
				continue
			}
			if e.Admin {
				continue
			}
			if !since.IsZero() && e.Time.Before(since) {
				continue
			}
			domain := hostToDomain[strings.ToLower(e.Domain)]
			if domain == "" {
				domain = hostToDomain[normalizeHost(e.Host)]
			}
			if domain == "" {
				continue
			}
			st, ok := out[domain]
			if !ok {
				st = SiteRangeStats{ByStatus: map[string]int64{}, ByReason: map[string]int64{}}
			}
			st.Total++
			st.ByStatus[statusLabel(e.Status)]++
			if e.Blocked {
				st.Blocked++
				r := e.BlockReason
				if r == "" {
					r = "unknown"
				}
				st.ByReason[r]++
			}
			out[domain] = st
		}
		_ = fh.Close()
	}
	for d, st := range out {
		st.Allowed = st.Total - st.Blocked
		out[d] = st
	}
	return out
}

// StatsFor aggregates persisted log entries since the given time (max 30s cache).
func (l *Logger) StatsFor(since time.Time) Stats {
	return l.statsFor(since, nil, true)
}

// StatsForDomains aggregates only entries for the given domains. Used for
// tenant-scoped dashboard statistics; not cached to avoid cross-tenant leaks.
func (l *Logger) StatsForDomains(since time.Time, domains map[string]bool) Stats {
	return l.statsFor(since, domains, false)
}

func (l *Logger) statsFor(since time.Time, domains map[string]bool, cache bool) Stats {
	l.mu.Lock()
	if cache && !l.windowCachedAt.IsZero() && time.Since(l.windowCachedAt) < 30*time.Second {
		st := l.windowStats
		l.mu.Unlock()
		return st
	}
	dir := l.dir
	l.mu.Unlock()

	st := Stats{ByStatus: map[string]int64{}, ByReason: map[string]int64{}}
	if dir == "" {
		return st
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	cutoffDate := today.AddDate(0, 0, -1)

	byStatusInt := map[int]int64{}
	byReason := map[string]int64{}
	sites := map[string]*SiteStat{}
	countries := map[string]int64{}
	var total, blocked int64

	files, _ := l.Files()
	for _, f := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(f.Name, "access-"), ".jsonl")
		d, err := time.ParseInLocation("2006-01-02", name, time.Local)
		if err == nil && d.Before(cutoffDate) {
			continue
		}
		fh, err := os.Open(filepath.Join(dir, f.Name))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			var e Entry
			if json.Unmarshal(sc.Bytes(), &e) != nil {
				continue
			}
			if e.Time.Before(since) || e.Admin {
				continue
			}
			total++
			byStatusInt[e.Status]++
			if e.Blocked {
				blocked++
				byReason[e.BlockReason]++
				if e.Country != "" {
					countries[e.Country]++
				}
			}
			if e.Domain != "" {
				ss := sites[e.Domain]
				if ss == nil {
					ss = &SiteStat{Domain: e.Domain}
					sites[e.Domain] = ss
				}
				ss.Total++
				if e.Blocked {
					ss.Blocked++
				}
			}
		}
		_ = fh.Close()
	}

	st.Total = total
	st.Blocked = blocked
	st.Allowed = total - blocked
	for code, n := range byStatusInt {
		st.ByStatus[statusLabel(code)] += n
	}
	for reason, n := range byReason {
		if reason == "" {
			reason = "unknown"
		}
		st.ByReason[reason] = n
	}
	sl := make([]SiteStat, 0, len(sites))
	for _, s := range sites {
		sl = append(sl, *s)
	}
	sort.Slice(sl, func(i, j int) bool { return sl[i].Total > sl[j].Total })
	if len(sl) > 10 {
		sl = sl[:10]
	}
	st.TopSites = sl
	cl := make([]CountryStat, 0, len(countries))
	for cc, n := range countries {
		cl = append(cl, CountryStat{Country: cc, Count: n})
	}
	sort.Slice(cl, func(i, j int) bool { return cl[i].Count > cl[j].Count })
	if len(cl) > 10 {
		cl = cl[:10]
	}
	st.BlockedCountries = cl
	if d := time.Since(since).Seconds(); d > 0 {
		st.RPS = float64(total) / d
	}

	l.mu.Lock()
	l.windowStats = st
	l.windowCachedAt = time.Now()
	l.mu.Unlock()
	return st
}

func (l *Logger) Stats() Stats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	st := Stats{
		UptimeSeconds: int64(time.Since(l.started).Seconds()),
		Total:         l.total,
		Blocked:       l.blocked,
		Allowed:       l.total - l.blocked,
		ByStatus:      map[string]int64{},
		ByReason:      map[string]int64{},
	}
	for code, n := range l.byStatus {
		st.ByStatus[statusLabel(code)] = n
	}
	for reason, n := range l.byReason {
		if reason == "" {
			reason = "unknown"
		}
		st.ByReason[reason] = n
	}
	sites := make([]SiteStat, 0, len(l.sites))
	for _, s := range l.sites {
		sites = append(sites, *s)
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].Total > sites[j].Total })
	if len(sites) > 10 {
		sites = sites[:10]
	}
	st.TopSites = sites

	countries := make([]CountryStat, 0, len(l.blockedCountries))
	for cc, n := range l.blockedCountries {
		countries = append(countries, CountryStat{Country: cc, Count: n})
	}
	sort.Slice(countries, func(i, j int) bool { return countries[i].Count > countries[j].Count })
	if len(countries) > 10 {
		countries = countries[:10]
	}
	st.BlockedCountries = countries

	since := l.statsSince
	if since.IsZero() {
		since = l.started
	}
	if d := time.Since(since).Seconds(); d > 0 {
		st.RPS = float64(st.Total) / d
	}
	return st
}

func readTail(path string, maxLines int) []Entry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	const chunk = 256 * 1024
	stat, err := f.Stat()
	if err != nil {
		return nil
	}
	size := stat.Size()
	offset := size
	var data []byte
	for offset > 0 && countNewlines(data) <= maxLines {
		readSize := int64(chunk)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize
		buf := make([]byte, readSize)
		if _, err := f.ReadAt(buf, offset); err != nil && err != io.EOF {
			return nil
		}
		data = append(buf, data...)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	out := make([]Entry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			out = append(out, e)
		}
	}
	return out
}

func countNewlines(b []byte) int {
	return strings.Count(string(b), "\n")
}

func statusLabel(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "other"
	}
}
