package dynacat

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
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

var dockerHealthPattern = regexp.MustCompile(`\((healthy|unhealthy|health: starting)\)`)

func (a *application) registerGoodiecatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/goodiecat/host", a.handleGoodiecatHost)
	mux.HandleFunc("GET /api/goodiecat/containers", a.handleGoodiecatContainers)
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
	selfID := getSelfContainerID()
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
