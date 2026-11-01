package dynacat

import (
	"cmp"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Panonim/dynacat/pkg/sysinfo"
)

// The `goodiecat:` section of the config. It powers the Goodiecat phone app
// (down alerts, tapping a service to start / stop / restart it) and the
// server card template, without any extra service next to the dashboard.
//
//	goodiecat:
//	  app-token: ${GOODIECAT_APP_TOKEN}   # needed for start/stop/restart
//	  docker:
//	    sock-path: /var/run/docker.sock   # or tcp://docker-socket-proxy:2375
//	    restart-only: [traefik]           # never stopped from the app
//	  host:                               # same options as a local server-stats server
//	    mountpoints:
//	      "/": { name: System }
type goodiecatConfig struct {
	AppToken string `yaml:"app-token"`
	Docker   struct {
		SockPath    string   `yaml:"sock-path"`
		RestartOnly []string `yaml:"restart-only"`
	} `yaml:"docker"`
	Host sysinfo.SystemInfoRequest `yaml:"host"`
}

const goodiecatTokenHeader = "X-Goodiecat-Token"

var (
	dockerHealthPattern = regexp.MustCompile(`\((healthy|unhealthy|health: starting)\)`)
	shortContainerID    = regexp.MustCompile(`^[0-9a-f]{12}$`)
)

// goodiecatSelfID finds this dashboard's own container. The cgroup file is
// empty under cgroup v2, but Docker names a container after its short ID
// unless a hostname is set.
func goodiecatSelfID() string {
	if id := getSelfContainerID(); id != "" {
		return id
	}
	if h, err := os.Hostname(); err == nil && shortContainerID.MatchString(h) {
		return h
	}
	return ""
}

func (a *application) registerGoodiecatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/goodiecat/host", a.handleGoodiecatHost)
	mux.HandleFunc("GET /api/goodiecat/containers", a.handleGoodiecatContainers)
	mux.HandleFunc("GET /api/goodiecat/containers/stats", a.handleGoodiecatContainerStats)
	mux.HandleFunc("POST /api/goodiecat/containers/{action}", a.handleGoodiecatControl)
}

// hasGoodiecatToken checks the app token in constant time; an unset token
// never matches.
func (a *application) hasGoodiecatToken(r *http.Request) bool {
	want := a.Config.Goodiecat.AppToken
	given := r.Header.Get(goodiecatTokenHeader)
	return want != "" && subtle.ConstantTimeCompare([]byte(given), []byte(want)) == 1
}

// Read-only endpoints follow the dashboard's own login, or accept the app token.
func (a *application) goodiecatCanRead(w http.ResponseWriter, r *http.Request) bool {
	if a.hasGoodiecatToken(r) {
		return true
	}
	return !a.handleUnauthorizedResponse(w, r, showUnauthorizedJSON)
}

func (a *application) goodiecatDocker() *dockerCtrlClient {
	sock := a.Config.Goodiecat.Docker.SockPath
	if sock == "" {
		sock = "/var/run/docker.sock"
	}
	return newDockerCtrlClient(sock)
}

func writeGoodiecatJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// GET /api/goodiecat/host: CPU, temperature, memory, disks and boot time,
// for the server card on the home page.
func (a *application) handleGoodiecatHost(w http.ResponseWriter, r *http.Request) {
	if !a.goodiecatCanRead(w, r) {
		return
	}
	info, errs := sysinfo.Collect(&a.Config.Goodiecat.Host)
	for _, err := range errs {
		slog.Debug("Collecting host info for goodiecat", "error", err)
	}
	// inside a container, report the host's name rather than the container's
	if etc := os.Getenv("HOST_ETC"); etc != "" {
		if name, err := os.ReadFile(filepath.Join(etc, "hostname")); err == nil && strings.TrimSpace(string(name)) != "" {
			info.Hostname = strings.TrimSpace(string(name))
		}
	}
	writeGoodiecatJSON(w, http.StatusOK, map[string]any{"ok": true, "data": info})
}

type goodiecatContainer struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Health string `json:"health"`
	Status string `json:"status"`
	// the app greys out Stop for these
	RestartOnly bool `json:"restart_only"`
}

// stopping the dashboard itself (or anything listed) would leave nothing on
// the phone to start it again with
func (a *application) isRestartOnly(c dockerCtrlContainerJSON) bool {
	selfID := goodiecatSelfID()
	if selfID != "" && strings.HasPrefix(c.ID, selfID) {
		return true
	}
	name := dockerContainerName(c)
	return slices.ContainsFunc(a.Config.Goodiecat.Docker.RestartOnly, func(n string) bool {
		return strings.EqualFold(n, name)
	})
}

func (a *application) listGoodiecatContainers() ([]dockerCtrlContainerJSON, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := a.goodiecatDocker()
	req, err := client.newRequest(ctx, "GET", "/containers/json?all=true")
	if err != nil {
		return nil, err
	}
	resp, err := client.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker API returned status %d", resp.StatusCode)
	}
	var raw []dockerCtrlContainerJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// GET /api/goodiecat/containers: names and states only, which is all the
// phone app's background check needs.
func (a *application) handleGoodiecatContainers(w http.ResponseWriter, r *http.Request) {
	if !a.goodiecatCanRead(w, r) {
		return
	}
	raw, err := a.listGoodiecatContainers()
	if err != nil {
		writeGoodiecatJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "Docker is not reachable"})
		return
	}
	out := make([]goodiecatContainer, 0, len(raw))
	for _, c := range raw {
		health := ""
		if m := dockerHealthPattern.FindStringSubmatch(c.Status); m != nil {
			health = strings.TrimPrefix(m[1], "health: ")
		}
		out = append(out, goodiecatContainer{
			Name:        dockerContainerName(c),
			Image:       c.Image,
			State:       strings.ToLower(c.State),
			Health:      health,
			Status:      c.Status,
			RestartOnly: a.isRestartOnly(c),
		})
	}
	writeGoodiecatJSON(w, http.StatusOK, map[string]any{"ok": true, "data": out})
}

// POST /api/goodiecat/containers/{start|stop|restart} with {"name", "image"}.
// The dashboard tile only knows its display name and image, so the container
// is matched by name first (case-insensitive), then by image.
func (a *application) handleGoodiecatControl(w http.ResponseWriter, r *http.Request) {
	if a.Config.Goodiecat.AppToken == "" {
		writeGoodiecatJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "Set goodiecat.app-token to allow this"})
		return
	}
	if !a.hasGoodiecatToken(r) {
		writeGoodiecatJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "Wrong app token"})
		return
	}
	action := r.PathValue("action")
	if action != "start" && action != "stop" && action != "restart" {
		writeGoodiecatJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Unknown action"})
		return
	}

	var body struct {
		Name  string `json:"name"`
		Image string `json:"image"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		writeGoodiecatJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Bad request body"})
		return
	}

	raw, err := a.listGoodiecatContainers()
	if err != nil {
		writeGoodiecatJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "Docker is not reachable"})
		return
	}
	var matches []dockerCtrlContainerJSON
	for _, c := range raw {
		if strings.EqualFold(dockerContainerName(c), strings.TrimSpace(body.Name)) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 && body.Image != "" {
		for _, c := range raw {
			if c.Image == body.Image {
				matches = append(matches, c)
			}
		}
	}
	if len(matches) != 1 {
		writeGoodiecatJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "No single container matches " + body.Name})
		return
	}
	target := matches[0]
	name := dockerContainerName(target)

	if action == "stop" && a.isRestartOnly(target) {
		writeGoodiecatJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": name + " can only be restarted"})
		return
	}

	if err := dockerCtrlContainerAction(a.goodiecatDocker(), target.ID, action); err != nil {
		writeGoodiecatJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	slog.Info("Container action from the Goodiecat app", "action", action, "container", name)
	writeGoodiecatJSON(w, http.StatusOK, map[string]any{"ok": true, "container": name, "action": action})
}

func dockerContainerName(c dockerCtrlContainerJSON) string {
	if len(c.Names) == 0 {
		return ""
	}
	return strings.TrimLeft(c.Names[0], "/")
}

type goodiecatContainerStats struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	State       string  `json:"state"`
	Health      string  `json:"health"`
	Status      string  `json:"status"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes uint64  `json:"memory_bytes"`
	MemoryLimit uint64  `json:"memory_limit"`
	// share of the machine's memory
	MemoryPercent float64 `json:"memory_percent"`
	// 0-100 against the busiest container, for bars that stay readable
	CPUBar    float64 `json:"cpu_bar"`
	MemoryBar float64 `json:"memory_bar"`
}

type dockerStatsJSON struct {
	CPUStats    dockerCPUStats `json:"cpu_stats"`
	PreCPUStats dockerCPUStats `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
}

type dockerCPUStats struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
}

var goodiecatStatsCache struct {
	sync.Mutex
	at         time.Time
	data       []goodiecatContainerStats
	refreshing bool
}

// GET /api/goodiecat/containers/stats: CPU and memory for every running
// container (stopped ones listed without numbers), busiest first.
//
// Docker samples each container for about a second, which with many
// containers is longer than a widget waits. So this always answers right
// away with the latest numbers and refreshes them in the background; on the
// very first call the list comes back without numbers.
func (a *application) handleGoodiecatContainerStats(w http.ResponseWriter, r *http.Request) {
	if !a.goodiecatCanRead(w, r) {
		return
	}
	cache := &goodiecatStatsCache
	cache.Lock()
	if time.Since(cache.at) > 10*time.Second && !cache.refreshing {
		cache.refreshing = true
		go func() {
			data, err := a.collectGoodiecatStats()
			cache.Lock()
			defer cache.Unlock()
			cache.refreshing = false
			if err == nil {
				cache.data, cache.at = data, time.Now()
			}
		}()
	}
	data := cache.data
	cache.Unlock()

	if data == nil {
		raw, err := a.listGoodiecatContainers()
		if err != nil {
			writeGoodiecatJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "Docker is not reachable"})
			return
		}
		data = goodiecatStatsSkeleton(raw)
	}
	writeGoodiecatJSON(w, http.StatusOK, map[string]any{"ok": true, "data": data})
}

func goodiecatStatsSkeleton(raw []dockerCtrlContainerJSON) []goodiecatContainerStats {
	out := make([]goodiecatContainerStats, len(raw))
	for i, c := range raw {
		name := dockerContainerName(c)
		health := ""
		if m := dockerHealthPattern.FindStringSubmatch(c.Status); m != nil {
			health = strings.TrimPrefix(m[1], "health: ")
		}
		out[i] = goodiecatContainerStats{
			Name:        name,
			DisplayName: formatDockerCtrlName(name),
			State:       strings.ToLower(c.State),
			Health:      health,
			Status:      c.Status,
		}
	}
	slices.SortStableFunc(out, func(x, y goodiecatContainerStats) int {
		if (x.State == "running") != (y.State == "running") {
			if x.State == "running" {
				return -1
			}
			return 1
		}
		return cmp.Compare(strings.ToLower(x.DisplayName), strings.ToLower(y.DisplayName))
	})
	return out
}

func (a *application) collectGoodiecatStats() ([]goodiecatContainerStats, error) {
	raw, err := a.listGoodiecatContainers()
	if err != nil {
		return nil, err
	}
	client := a.goodiecatDocker()
	out := goodiecatStatsSkeleton(raw)
	ids := make(map[string]string, len(raw))
	for _, c := range raw {
		ids[dockerContainerName(c)] = c.ID
	}
	var wg sync.WaitGroup
	limit := make(chan struct{}, 16)
	for i := range out {
		if out[i].State != "running" {
			continue
		}
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			limit <- struct{}{}
			defer func() { <-limit }()
			s, err := fetchDockerStats(client, id)
			if err != nil {
				return
			}
			out[i].CPUPercent, out[i].MemoryBytes, out[i].MemoryLimit = s.cpu, s.mem, s.limit
			if s.limit > 0 {
				out[i].MemoryPercent = float64(s.mem) / float64(s.limit) * 100
			}
		}(i, ids[out[i].Name])
	}
	wg.Wait()

	var maxCPU, maxMem float64
	for _, s := range out {
		maxCPU = max(maxCPU, s.CPUPercent)
		maxMem = max(maxMem, float64(s.MemoryBytes))
	}
	for i := range out {
		if maxCPU > 0 {
			out[i].CPUBar = out[i].CPUPercent / maxCPU * 100
		}
		if maxMem > 0 {
			out[i].MemoryBar = float64(out[i].MemoryBytes) / maxMem * 100
		}
	}

	slices.SortStableFunc(out, func(x, y goodiecatContainerStats) int {
		// running first, then by CPU, then memory
		if (x.State == "running") != (y.State == "running") {
			if x.State == "running" {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(y.CPUPercent, x.CPUPercent); c != 0 {
			return c
		}
		return cmp.Compare(y.MemoryBytes, x.MemoryBytes)
	})
	return out, nil
}

type dockerStatsSummary struct {
	cpu        float64
	mem, limit uint64
}

func fetchDockerStats(client *dockerCtrlClient, id string) (dockerStatsSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := client.newRequest(ctx, "GET", "/containers/"+id+"/stats?stream=false")
	if err != nil {
		return dockerStatsSummary{}, err
	}
	resp, err := client.http.Do(req)
	if err != nil {
		return dockerStatsSummary{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return dockerStatsSummary{}, fmt.Errorf("docker API returned status %d", resp.StatusCode)
	}
	var s dockerStatsJSON
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return dockerStatsSummary{}, err
	}

	// share of the whole machine (0-100), unlike `docker stats`, which
	// counts per core and goes past 100
	var cpu float64
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)
	if cpuDelta > 0 && sysDelta > 0 {
		cpu = cpuDelta / sysDelta * 100
	}
	mem := s.MemoryStats.Usage
	// page cache isn't really "used" (cgroup v2 calls it inactive_file)
	if cache := s.MemoryStats.Stats["inactive_file"]; cache < mem {
		mem -= cache
	} else if cache := s.MemoryStats.Stats["total_inactive_file"]; cache < mem {
		mem -= cache
	}
	return dockerStatsSummary{cpu: cpu, mem: mem, limit: s.MemoryStats.Limit}, nil
}
