// Command que-huong-studio is a local web workspace for the suno-que-huong skill:
// it drives each song from lyrics to karaoke video with scripts and coding agents.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var version = "1.0.0"

func main() {
	home, _ := os.UserHomeDir()
	var (
		configPath = flag.String("config", filepath.Join(home, ".config", "que-huong-studio", "config.json"), "config file")
		dataDir    = flag.String("data", filepath.Join(home, ".local", "share", "que-huong-studio"), "run history directory")
		addr       = flag.String("addr", "", "listen address (default from config, 127.0.0.1:4317)")
		noOpen     = flag.Bool("no-open", false, "do not open the browser")
		showVer    = flag.Bool("version", false, "print version")
	)
	flag.Parse()
	if *showVer {
		fmt.Println(version)
		return
	}
	cfg, err := loadConfig(*configPath, home)
	if err != nil {
		fatal(err)
	}
	if *addr != "" {
		c := cfg.Get()
		c.Addr = *addr
		cfg.cfg = c
	}
	listen := cfg.Get().Addr
	if host, _, err := net.SplitHostPort(listen); err != nil || (host != "127.0.0.1" && host != "localhost" && host != "::1") {
		fatal(fmt.Errorf("refusing to listen on %q: the studio is local-only", listen))
	}
	url := "http://" + listen + "/"

	// One studio per port: a second launch just brings the running one to the front.
	if alreadyRunning(listen) {
		fmt.Println("Quê Hương Studio đang chạy:", url)
		if !*noOpen {
			_ = exec.Command("open", url).Run()
		}
		return
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		fatal(err)
	}
	runs, err := newManager(*dataDir, func() int { return cfg.Get().MaxConcurrent })
	if err != nil {
		fatal(err)
	}
	app := &App{cfg: cfg, runs: runs, projects: Projects{cfg: cfg}, token: newToken(), home: home, version: version, build: webBuild(), started: time.Now()}
	runs.OnFinish = func(m RunMeta) {
		if m.Status == "succeeded" && m.InputsHash != "" && m.Project != "" {
			_ = app.projects.RecordStamp(m.Project, m.Step, m.InputsHash)
		}
	}
	srv := &http.Server{Handler: app.routes(), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal(err)
		}
	}()
	fmt.Println("Quê Hương Studio:", url)
	if !*noOpen {
		_ = exec.Command("open", url).Run()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nĐang dừng các lượt chạy…")
	runs.Shutdown(8 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func alreadyRunning(addr string) bool {
	client := http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Get("http://" + addr + "/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var h struct {
		App string `json:"app"`
	}
	return json.NewDecoder(resp.Body).Decode(&h) == nil && h.App == "que-huong-studio"
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "que-huong-studio:", err)
	os.Exit(1)
}
