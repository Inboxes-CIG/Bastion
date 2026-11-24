package system

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Info struct {
	CPUPercent float64 `json:"cpu_percent"`
	RAMUsed    uint64  `json:"ram_used"`
	RAMTotal   uint64  `json:"ram_total"`
	DiskUsed   uint64  `json:"disk_used"`
	DiskTotal  uint64  `json:"disk_total"`
	Load1      float64 `json:"load1"`
	Load5      float64 `json:"load5"`
	Load15     float64 `json:"load15"`
	Uptime     int64   `json:"uptime_seconds"`
}

var (
	mu        sync.Mutex
	prevTotal uint64
	prevIdle  uint64
	primed    bool
)

func cpuSample() (total, idle uint64, ok bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	var vals []uint64
	for _, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		vals = append(vals, v)
	}
	for _, v := range vals {
		total += v
	}
	idle = vals[3]
	if len(vals) > 4 {
		idle += vals[4]
	}
	return total, idle, true
}

func ramInfo() (used, total uint64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var memTotal, memAvail uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			v, _ := strconv.ParseUint(fields[1], 10, 64)
			memTotal = v * 1024
		case "MemAvailable:":
			v, _ := strconv.ParseUint(fields[1], 10, 64)
			memAvail = v * 1024
		}
	}
	if memTotal == 0 {
		return 0, 0
	}
	return memTotal - memAvail, memTotal
}

func diskInfo() (used, total uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		return 0, 0
	}
	bs := uint64(st.Bsize)
	total = st.Blocks * bs
	used = (st.Blocks - st.Bfree) * bs
	return used, total
}

func loadAvg() (l1, l5, l15 float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		l1, _ = strconv.ParseFloat(fields[0], 64)
		l5, _ = strconv.ParseFloat(fields[1], 64)
		l15, _ = strconv.ParseFloat(fields[2], 64)
	}
	return
}

func uptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		if f, err := strconv.ParseFloat(fields[0], 64); err == nil {
			return int64(f)
		}
	}
	return 0
}

// Snapshot returns current system resource usage. CPU percent is computed since
// the previous call (call it periodically, e.g. from the dashboard).
func Snapshot() Info {
	info := Info{}

	mu.Lock()
	total, idle, ok := cpuSample()
	if ok {
		if primed && total > prevTotal {
			dt := total - prevTotal
			di := idle - prevIdle
			if dt > 0 {
				info.CPUPercent = float64(dt-di) / float64(dt) * 100
			}
		}
		prevTotal = total
		prevIdle = idle
		primed = true
	}
	mu.Unlock()

	info.RAMUsed, info.RAMTotal = ramInfo()
	info.DiskUsed, info.DiskTotal = diskInfo()
	info.Load1, info.Load5, info.Load15 = loadAvg()
	info.Uptime = uptime()
	return info
}

// Prime initializes the CPU baseline (call once at startup).
func Prime() {
	_ = time.Now()
	mu.Lock()
	total, idle, ok := cpuSample()
	if ok {
		prevTotal = total
		prevIdle = idle
		primed = true
	}
	mu.Unlock()
}
