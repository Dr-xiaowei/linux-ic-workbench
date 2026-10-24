package workbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Instance struct {
	URL     string `json:"url"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
	UID     int    `json:"uid"`
	Version string `json:"version"`
	Mode    string `json:"mode"`
	Started string `json:"started"`
}

func ListenLoopback(address string, fallback bool) (net.Listener, error) {
	if address == "auto" {
		address = "127.0.0.1:0"
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || (host != "127.0.0.1" && host != "::1") {
		return nil, errors.New("listen must use a loopback IP; remote use via SSH forwarding")
	}
	ln, err := net.Listen("tcp", address)
	// Old personal configs retain :8765. They may fall back without rewriting config.
	// An explicit --listen never falls back, so SSH/automation cannot silently misroute.
	if err != nil && fallback && errors.Is(err, syscall.EADDRINUSE) {
		return net.Listen("tcp", net.JoinHostPort(host, "0"))
	}
	return ln, err
}

func (a *App) PublishInstance(address string) (Instance, error) {
	v := Instance{URL: "http://" + address, Token: a.Token, PID: os.Getpid(), UID: os.Geteuid(), Version: Version, Mode: a.Config.Mode, Started: Now()}
	b, err := json.Marshal(v)
	if err == nil {
		err = atomicFile(filepath.Join(a.Store.dir, ".instance.json"), b)
	}
	return v, err
}

func (a *App) RemoveInstance() { os.Remove(filepath.Join(a.Store.dir, ".instance.json")) }

// A stale PID is not proof of a running instance. Verify a fresh authenticated
// response, and never redirect credentials to a URL outside loopback.
func RunningInstance(dir, mode string) (Instance, error) {
	var v Instance
	if mode == "production" {
		if _, err := PrivateDir(dir, false); err != nil {
			return v, err
		}
		if err := PrivateFile(filepath.Join(dir, ".instance.json")); err != nil {
			return v, err
		}
	}
	b, err := readBounded(filepath.Join(dir, ".instance.json"), 4096)
	if err != nil {
		return v, err
	}
	if err = json.Unmarshal(b, &v); err != nil {
		return v, err
	}
	if v.UID != os.Geteuid() || v.Mode != mode || len(v.Token) != 48 {
		return v, errors.New("instance identity mismatch")
	}
	req, err := http.NewRequest("GET", v.URL+"/api/state", nil)
	if err != nil {
		return v, err
	}
	if req.URL.Scheme != "http" || req.URL.User != nil || req.URL.Path != "/api/state" || req.URL.RawQuery != "" || req.URL.Fragment != "" || (req.URL.Hostname() != "127.0.0.1" && req.URL.Hostname() != "::1") {
		return v, errors.New("invalid instance loopback URL")
	}
	req.Header.Set("Authorization", "Bearer "+v.Token)
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return v, fmt.Errorf("instance is not responding (stale startup record): %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return v, errors.New("instance authentication failed; restart from your own terminal")
	}
	return v, nil
}
