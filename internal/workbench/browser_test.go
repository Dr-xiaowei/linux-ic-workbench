package workbench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserLaunchBoundaryAndSingleArgument(t *testing.T) {
	trueBin, _ := exec.LookPath("true")
	falseBin, _ := exec.LookPath("false")
	v := Instance{URL: "http://127.0.0.1:12345", Token: strings.Repeat("a", 48)}
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	if _, _, e := browserCommand(v, trueBin, "linux"); e == nil {
		t.Fatal("headless launch allowed")
	}
	t.Setenv("DISPLAY", ":99")
	p, args, e := browserCommand(v, trueBin, "linux")
	if e != nil || p != trueBin || len(args) != 1 || args[0] != v.URL+"/#token="+v.Token {
		t.Fatal(p, args, e)
	}
	for _, bad := range []string{"https://external.example", "http://127.0.0.1:4/path", "http://u@127.0.0.1:4", "http://127.0.0.1:4/?other=1"} {
		x := v
		x.URL = bad
		if _, _, e := browserCommand(x, trueBin, "linux"); e == nil {
			t.Fatal(bad)
		}
	}
	t.Setenv("ICW_BROWSER", "/bin/true; touch /tmp/never")
	if _, _, e := browserCommand(v, "", "linux"); e == nil {
		t.Fatal("shell template accepted")
	}
	if e := OpenBrowser(v, falseBin); e == nil {
		t.Fatal("failed opener reported success")
	}
	dir := privateTestDir(t)
	exe := filepath.Join(dir, "browser with spaces")
	record := filepath.Join(dir, "args")
	if e := os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s\\n' \"$#\" \"$1\" > \"$ICW_BROWSER_TEST_RECORD\"\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("ICW_BROWSER_TEST_RECORD", record)
	if e := OpenBrowser(v, exe); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(record)
	if e != nil || string(b) != "1\n"+args[0]+"\n" {
		t.Fatal(string(b), e)
	}
}
