package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/response"
)

const metricsInterval = 7 * time.Second

// SystemHandler streams OS and Go runtime metrics via SSE.
type SystemHandler struct {
	rdb       *redis.Client
	startTime time.Time
	cpuModel  string
	log       zerolog.Logger

	// CPU delta state
	prevIdle  uint64
	prevTotal uint64
}

func NewSystemHandler(rdb *redis.Client, log zerolog.Logger) *SystemHandler {
	h := &SystemHandler{
		rdb:       rdb,
		startTime: time.Now(),
		cpuModel:  readCPUModel(),
		log:       log.With().Str("component", "system_handler").Logger(),
	}
	// Seed initial CPU reading so the first tick gets a real delta
	h.prevIdle, h.prevTotal, _ = readCPUStat()
	return h
}

// ---------- SSE Endpoint ----------

type systemMetrics struct {
	Timestamp int64  `json:"timestamp"`
	Uptime    string `json:"uptime"`

	// OS
	CPUPercent     float64 `json:"cpu_percent"`
	MemUsedBytes   uint64  `json:"mem_used_bytes"`
	MemTotalBytes  uint64  `json:"mem_total_bytes"`
	MemPercent     float64 `json:"mem_percent"`
	DiskUsedBytes  uint64  `json:"disk_used_bytes"`
	DiskTotalBytes uint64  `json:"disk_total_bytes"`
	DiskPercent    float64 `json:"disk_percent"`
	LoadAvg1       float64 `json:"load_avg_1"`
	LoadAvg5       float64 `json:"load_avg_5"`
	LoadAvg15      float64 `json:"load_avg_15"`

	// Go Application
	Goroutines  int    `json:"goroutines"`
	HeapAlloc   uint64 `json:"heap_alloc"`
	HeapSys     uint64 `json:"heap_sys"`
	StackInuse  uint64 `json:"stack_inuse"`
	NumGC       uint32 `json:"num_gc"`
	AppRSSBytes uint64 `json:"app_rss_bytes"`
	GoVersion   string `json:"go_version"`
	NumCPU      int    `json:"num_cpu"`
	CPUModel    string `json:"cpu_model"`

	// Worker Queues
	QueueAnswers       int64 `json:"queue_answers"`
	QueueCheats        int64 `json:"queue_cheats"`
	QueueScores        int64 `json:"queue_scores"`
	QueueQuestionOrder int64 `json:"queue_question_order"`
}

// SystemMetricsSSE godoc
// GET /api/v1/admin/system/metrics
func (h *SystemHandler) SystemMetricsSSE(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	reqCtx := c.Request.Context()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	h.log.Info().Msg("Admin connected to system metrics SSE")

	ticker := time.NewTicker(metricsInterval)
	defer ticker.Stop()

	// Send immediately on connect, then every tick
	h.writeMetrics(c)

	for {
		select {
		case <-reqCtx.Done():
			h.log.Info().Msg("Admin disconnected from system metrics SSE")
			return
		case <-ticker.C:
			h.writeMetrics(c)
		}
	}
}

func (h *SystemHandler) writeMetrics(c *gin.Context) {
	m := h.collect()
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	c.Writer.Write([]byte("data: "))
	c.Writer.Write(data)
	c.Writer.Write([]byte("\n\n"))
	c.Writer.Flush()
}

func (h *SystemHandler) collect() systemMetrics {
	m := systemMetrics{
		Timestamp: time.Now().Unix(),
		Uptime:    formatDuration(time.Since(h.startTime)),
		GoVersion: runtime.Version(),
		NumCPU:    runtime.NumCPU(),
		CPUModel:  h.cpuModel,
	}

	// ── CPU ──
	idle, total, err := readCPUStat()
	if err == nil && total > h.prevTotal {
		idleDelta := float64(idle - h.prevIdle)
		totalDelta := float64(total - h.prevTotal)
		m.CPUPercent = (1 - idleDelta/totalDelta) * 100
		h.prevIdle = idle
		h.prevTotal = total
	}

	// ── Memory ──
	memTotal, memAvail, err := readMemInfo()
	if err == nil && memTotal > 0 {
		m.MemTotalBytes = memTotal
		m.MemUsedBytes = memTotal - memAvail
		m.MemPercent = float64(m.MemUsedBytes) / float64(memTotal) * 100
	}

	// ── Disk ──
	diskTotal, diskFree, err := readDisk("/")
	if err == nil && diskTotal > 0 {
		m.DiskTotalBytes = diskTotal
		m.DiskUsedBytes = diskTotal - diskFree
		m.DiskPercent = float64(m.DiskUsedBytes) / float64(diskTotal) * 100
	}

	// ── Load Average ──
	m.LoadAvg1, m.LoadAvg5, m.LoadAvg15, _ = readLoadAvg()

	// ── Go Runtime ──
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m.Goroutines = runtime.NumGoroutine()
	m.HeapAlloc = ms.HeapAlloc
	m.HeapSys = ms.Sys
	m.StackInuse = ms.StackInuse
	m.NumGC = ms.NumGC

	// ── App RSS ──
	m.AppRSSBytes, _ = readProcessRSS()

	// ── Worker Queues (pipelined LLEN) ──
	ctx := context.Background()
	pipe := h.rdb.Pipeline()
	answersCmd := pipe.LLen(ctx, config.WorkerKey.PersistAnswersQueue)
	cheatsCmd := pipe.LLen(ctx, config.WorkerKey.PersistCheatsQueue)
	scoresCmd := pipe.LLen(ctx, config.WorkerKey.PersistScoresQueue)
	orderCmd := pipe.LLen(ctx, config.WorkerKey.PersistQuestionOrderQueue)
	if _, err := pipe.Exec(ctx); err == nil {
		m.QueueAnswers, _ = answersCmd.Result()
		m.QueueCheats, _ = cheatsCmd.Result()
		m.QueueScores, _ = scoresCmd.Result()
		m.QueueQuestionOrder, _ = orderCmd.Result()
	}

	return m
}

// ---------- /proc Readers ----------

// readCPUStat parses /proc/stat for aggregate CPU times.
// Returns idle ticks and total ticks.
func readCPUStat() (idle, total uint64, err error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	// First line: cpu  user nice system idle iowait irq softirq steal ...
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("unexpected /proc/stat format")
	}

	for i := 1; i < len(fields); i++ {
		val, _ := strconv.ParseUint(fields[i], 10, 64)
		total += val
		if i == 4 { // idle is the 4th value (0-indexed field 4)
			idle = val
		}
	}
	return idle, total, nil
}

// readCPUModel parses /proc/cpuinfo to extract the "model name".
func readCPUModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "Unknown"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "Unknown"
}

// readMemInfo parses /proc/meminfo for MemTotal and MemAvailable.
func readMemInfo() (total, available uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	found := 0
	for scanner.Scan() && found < 2 {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			total = parseMemInfoValue(line)
			found++
		} else if strings.HasPrefix(line, "MemAvailable:") {
			available = parseMemInfoValue(line)
			found++
		}
	}
	return total, available, nil
}

func parseMemInfoValue(line string) uint64 {
	// Format: "MemTotal:       16384000 kB"
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	val, _ := strconv.ParseUint(fields[1], 10, 64)
	return val * 1024 // Convert kB to bytes
}

// readDisk uses syscall.Statfs to get disk usage.
func readDisk(path string) (total, free uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	total = stat.Blocks * uint64(stat.Bsize)
	free = stat.Bavail * uint64(stat.Bsize)
	return total, free, nil
}

// readLoadAvg parses /proc/loadavg.
func readLoadAvg() (load1, load5, load15 float64, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg format")
	}
	load1, _ = strconv.ParseFloat(fields[0], 64)
	load5, _ = strconv.ParseFloat(fields[1], 64)
	load15, _ = strconv.ParseFloat(fields[2], 64)
	return load1, load5, load15, nil
}

// readProcessRSS reads VmRSS from /proc/self/status.
func readProcessRSS() (uint64, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			return parseMemInfoValue(line), nil
		}
	}
	return 0, fmt.Errorf("VmRSS not found")
}

// ---------- Helpers ----------

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
