package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"icworkbench/internal/workbench"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "IC Workbench:", err)
		os.Exit(1)
	}
}

func run() error {
	config := flag.String("config", "", "configuration JSON")
	template := flag.String("config-template", "", "initialize missing private config from this template")
	mode := flag.String("mode", "production", "demo or production")
	data := flag.String("data", "", "persistent data directory")
	listen := flag.String("listen", "", "loopback listen address")
	isMCP := flag.Bool("mcp", false, "serve MCP over stdio; connect to running workbench")
	endpoint := flag.String("url", "", "workbench URL for MCP; defaults to this user's running instance")
	run := flag.Bool("allow-run", false, "enable MCP EDA execution tools")
	version := flag.Bool("version", false, "print build/runtime")
	check := flag.Bool("self-check", false, "check OS baseline and configuration without EDA")
	status := flag.Bool("status", false, "show this user's running instance and login URL")
	desktopAPI := flag.Bool("desktop-api", false, "native desktop service; do not serve a web interface")
	openGUI := flag.Bool("open", false, "open GUI in desktop browser; reuse this user's running instance")
	browser := flag.String("browser", "", "browser executable path for --open (no shell arguments)")
	flag.Parse()
	if *browser != "" && !*openGUI {
		return errors.New("--browser requires --open")
	}
	if *openGUI && (*isMCP || *check || *version) {
		return errors.New("--open cannot be combined with --mcp, --self-check or --version")
	}
	if flag.NArg() != 0 {
		return errors.New("unexpected arguments; use --help")
	}
	if *version {
		fmt.Println(workbench.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c := workbench.Defaults()
	if *config != "" {
		var e error
		initFrom := *template
		if *status {
			initFrom = ""
		}
		c, e = workbench.LoadConfig(*config, initFrom)
		if e != nil {
			return e
		}
	}
	explicitListen := false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "mode":
			c.Mode = *mode
		case "data":
			c.DataDir = *data
		case "listen":
			c.Listen = *listen
			explicitListen = true
		}
	})
	if c.DataDir == "" {
		home, e := os.UserHomeDir()
		if e != nil {
			return e
		}
		c.DataDir = filepath.Join(home, ".local", "share", "ic-workbench", c.Mode)
	}
	c.DesktopAPI = *desktopAPI
	if *desktopAPI && *openGUI {
		return errors.New("desktop API cannot open a browser; use the installed desktop command")
	}
	if *isMCP {
		token := os.Getenv("ICW_TOKEN")
		if *endpoint == "" {
			v, err := workbench.RunningInstance(c.DataDir, c.Mode)
			if err != nil {
				return fmt.Errorf("MCP needs a running instance; start the workbench first: %w", err)
			}
			*endpoint, token = v.URL, v.Token
		} else if token == "" {
			return errors.New("set ICW_TOKEN when using an explicit MCP --url")
		}
		return workbench.MCPServer(*endpoint, token, *run).Run(ctx, &mcp.StdioTransport{})
	}
	if c.Listen == "auto" {
		c.Listen = "127.0.0.1:0"
	}
	h, _, e := net.SplitHostPort(c.Listen)
	if e != nil || (h != "127.0.0.1" && h != "::1") {
		return errors.New("listen must use a loopback IP; remote use via SSH forwarding")
	}
	if *status {
		v, err := workbench.RunningInstance(c.DataDir, c.Mode)
		if err != nil {
			return fmt.Errorf("not running / 未运行: %w", err)
		}
		printInstance(v, c.DataDir)
		if *openGUI {
			return requestGUI(v, *browser)
		}
		return nil
	}
	if *check {
		fmt.Println("version:", workbench.Version, "runtime:", runtime.Version(), "OS:", runtime.GOOS, "arch:", runtime.GOARCH)
		if runtime.GOOS == "linux" {
			b, e := os.ReadFile("/proc/sys/kernel/osrelease")
			if e != nil {
				return e
			}
			k := strings.TrimSpace(string(b))
			fmt.Println("kernel:", k)
			parts := strings.Split(strings.Split(k, "-")[0], ".")
			if len(parts) >= 2 {
				major, _ := strconv.Atoi(parts[0])
				minor, _ := strconv.Atoi(parts[1])
				patch := 0
				if len(parts) > 2 {
					patch, _ = strconv.Atoi(parts[2])
				}
				if major < 2 || major == 2 && (minor < 6 || minor == 6 && patch < 32) {
					return errors.New("unsupported kernel")
				}
			}
		}
		fmt.Println("mode:", c.Mode, "data:", c.DataDir, "GLM configured:", c.ModelURL != "", "EDA adapter configured:", c.Probe.Executable != "")
		return nil
	}
	app, e := workbench.NewApp(c)
	if e != nil {
		if errors.Is(e, workbench.ErrStoreInUse) {
			// First startup may still be publishing its receipt; wait at most one second.
			for i := 0; i < 10; i++ {
				v, err := workbench.RunningInstance(c.DataDir, c.Mode)
				if err == nil {
					fmt.Fprintln(os.Stderr, "Already running / 已启动；复用当前服务")
					printInstance(v, c.DataDir)
					if *openGUI {
						return requestGUI(v, *browser)
					}
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		return e
	}
	defer app.Close()
	ln, e := workbench.ListenLoopback(c.Listen, !explicitListen)
	if e != nil {
		return fmt.Errorf("cannot listen on %s; use --listen auto or a free port: %w", c.Listen, e)
	}
	defer ln.Close()
	app.Config.Listen = ln.Addr().String()
	v, e := app.PublishInstance(ln.Addr().String())
	if e != nil {
		return e
	}
	defer app.RemoveInstance()
	app.StartValidation()
	srv := app.Server()
	printInstance(v, c.DataDir)
	guiDone := make(chan struct{})
	if *openGUI {
		go func() {
			defer close(guiDone)
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
				select {
				case <-ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
				}
				if _, e := workbench.RunningInstance(c.DataDir, c.Mode); e == nil {
					if e = requestGUI(v, *browser); e != nil {
						fmt.Fprintln(os.Stderr, "GUI:", e, "; backend remains available / 后台仍可访问")
					}
					return
				}
			}
			fmt.Fprintln(os.Stderr, "GUI: backend readiness timeout; open the printed URL manually / 请手动打开终端网址")
		}()
	} else {
		close(guiDone)
	}
	shutdownDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		app.BeginShutdown()
		deadline, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(deadline); err != nil {
			srv.Close()
		}
		close(shutdownDone)
	}()
	if e = srv.Serve(ln); e != nil && !errors.Is(e, http.ErrServerClosed) {
		stop()
		<-shutdownDone
		return e
	}
	<-shutdownDone
	<-guiDone
	return nil
}

func printInstance(v workbench.Instance, dir string) {
	fmt.Fprintf(os.Stderr, "IC Workbench %s [%s] UID=%d PID=%d\nOpen %s/#token=%s\nData: %s\nMCP token: export ICW_TOKEN=%s\n", v.Version, v.Mode, v.UID, v.PID, v.URL, v.Token, dir, v.Token)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(v.URL, "http://"))
	if host == "::1" {
		host = "[::1]"
	}
	fmt.Fprintf(os.Stderr, "Keep the startup terminal open; Ctrl+C stops this instance.\n保持启动终端打开，Ctrl+C 停止本实例。\nRemote browser / 远程浏览器: ssh -N -L 18765:%s:%s USER@SERVER\nThen open http://127.0.0.1:18765/ and use the token above.\n", host, port)
}

func requestGUI(v workbench.Instance, browser string) error {
	if e := workbench.OpenBrowser(v, browser); e != nil {
		return e
	}
	fmt.Fprintln(os.Stderr, "Browser open requested / 已请求浏览器打开 GUI；若窗口未显示，请使用上方网址。")
	return nil
}
