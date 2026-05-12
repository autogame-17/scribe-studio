//go:build smoke

// smoke test for sphkit: Start → probe → Stop, independent of the Wails UI.
// Run with: go run -tags smoke ./scripts/smoke/main.go
package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"wx_channel/pkg/sphkit"
)

func main() {
	fmt.Println("==> sphkit.New")
	inst, err := sphkit.New("0.0.0-smoke", "debug")
	if err != nil {
		fail("New failed: %v", err)
	}

	fmt.Println("==> Start")
	if err := inst.Start(); err != nil {
		fail("Start failed: %v", err)
	}
	defer func() {
		fmt.Println("==> Stop")
		if err := inst.Stop(); err != nil {
			fmt.Printf("Stop error: %v\n", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	st := inst.Status()
	fmt.Printf("    running=%v  interceptor=%s  api=%s  err=%q\n",
		st.Running, st.InterceptorAddr, st.APIAddr, st.LastError)

	for _, label := range []struct{ name, addr string }{
		{"interceptor", st.InterceptorAddr},
		{"api", st.APIAddr},
	} {
		if label.addr == "" {
			fmt.Printf("    [%s] no address\n", label.name)
			continue
		}
		conn, err := net.DialTimeout("tcp", label.addr, 2*time.Second)
		if err != nil {
			fmt.Printf("    [%s] %s  -- DIAL FAIL: %v\n", label.name, label.addr, err)
			continue
		}
		_ = conn.Close()
		fmt.Printf("    [%s] %s  -- OK\n", label.name, label.addr)
	}

	if st.Running {
		fmt.Println("==> PASS — backend chain is up")
	} else {
		fail("sphkit reports not running")
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "==> FAIL: "+format+"\n", a...)
	os.Exit(1)
}
