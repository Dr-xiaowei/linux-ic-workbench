package workbench

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Browser launch is explicit and never evaluates a shell command or BROWSER template.
func browserCommand(v Instance, browser, system string) (string, []string, error) {
	u, e := url.Parse(v.URL)
	if e != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || len(v.Token) != 48 {
		return "", nil, errors.New("invalid local GUI address")
	}
	if system == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return "", nil, errors.New("no graphical session (DISPLAY/WAYLAND_DISPLAY); use SSH forwarding and open the printed URL in your computer's browser / 无图形会话，请使用终端提供的 SSH 转发方式")
	}
	if browser == "" {
		browser = os.Getenv("ICW_BROWSER")
	}
	candidates := []string{browser}
	if browser == "" {
		if system == "darwin" {
			candidates = []string{"open"}
		} else {
			candidates = []string{"xdg-open", "firefox", "chromium", "chromium-browser", "google-chrome"}
		}
	}
	for _, candidate := range candidates {
		if p, e := exec.LookPath(candidate); e == nil {
			return p, []string{v.URL + "/#token=" + v.Token}, nil
		}
	}
	return "", nil, errors.New("no graphical browser launcher found; set --browser /absolute/path/to/browser or open the printed URL manually / 未找到浏览器，请指定 --browser 或手动打开网址")
}

// Returns an immediate failure when possible. A live GUI process is allowed to
// outlive this short request; reaping it must not close the user's browser.
func OpenBrowser(v Instance, browser string) error {
	p, args, e := browserCommand(v, browser, runtime.GOOS)
	if e != nil {
		return e
	}
	cmd := exec.Command(p, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Detached from startup terminal; no inherited stdout pipe or secret-bearing logs.
	if e = cmd.Start(); e != nil {
		return fmt.Errorf("browser launch failed: %w", e)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case e = <-done:
		if e != nil {
			return fmt.Errorf("browser launcher failed (%s); open the printed URL manually", strings.TrimSpace(e.Error()))
		}
	case <-time.After(800 * time.Millisecond):
	}
	return nil
}
