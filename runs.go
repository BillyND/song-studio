package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// RunSpec is what a caller asks the manager to execute.
type RunSpec struct {
	Kind    string `json:"kind"`  // "script" or "agent"
	Title   string `json:"title"` // shown in the UI
	Step    string `json:"step,omitempty"`
	Project string `json:"project,omitempty"`
	Agent   string `json:"agent,omitempty"`
	Model   string `json:"model,omitempty"`
	Prompt  string `json:"prompt,omitempty"`
	Cwd     string `json:"cwd"`
	// InputsHash fingerprints what a stills/render run was built from, so the
	// workflow can tell whether its output is still current.
	InputsHash string `json:"inputsHash,omitempty"`
	// FingerprintDir, when set, makes the run record InputsHash for that project as it starts.
	FingerprintDir string   `json:"fingerprintDir,omitempty"`
	Argv           []string `json:"argv"`
	// Heavy runs (vocal separation, rendering, audio analysis) share a small CPU budget;
	// light runs (agents waiting on an API, image downloads) run alongside them.
	Heavy bool `json:"heavy,omitempty"`
	// After lists run IDs that must succeed before this one starts; Group ties a chain together.
	After []string `json:"after,omitempty"`
	Group string   `json:"group,omitempty"`
	// Secrets names the credentials this run needs ("image"); they are resolved into the
	// child's environment when it starts and are never stored with the run.
	Secrets []string `json:"secrets,omitempty"`
	// TimeoutSec ends a run that takes longer; 0 picks a default from the kind of run.
	TimeoutSec int `json:"timeoutSec,omitempty"`
}

// RunMeta is the persisted, JSON-visible state of a run.
type RunMeta struct {
	RunSpec
	ID       string    `json:"id"`
	Status   string    `json:"status"` // queued, running, succeeded, failed, stopped, interrupted
	PID      int       `json:"pid,omitempty"`
	Created  time.Time `json:"created"`
	Started  time.Time `json:"started,omitempty"`
	Ended    time.Time `json:"ended,omitempty"`
	ExitCode *int      `json:"exitCode,omitempty"`
	Usage    Usage     `json:"usage"`
	Events   int       `json:"events"`
	Error    string    `json:"error,omitempty"`
}

func (m RunMeta) active() bool { return m.Status == "queued" || m.Status == "running" }

type run struct {
	mu      sync.Mutex
	meta    RunMeta
	events  []Event // kept in memory while the run is live and for recent history
	subs    map[chan Event]struct{}
	cmd     *exec.Cmd
	stopped bool
	// timedOut, requeue and interrupted say why a stopped run ended: past its time limit,
	// or the studio quit (a script runs again on the next start, an agent does not).
	timedOut, requeue, interrupted bool
	logFile                        *os.File
	done                           chan struct{}
}

// Manager queues and executes runs, persisting each under dataDir/runs/<id>.
type Manager struct {
	mu    sync.Mutex
	dir   string
	runs  map[string]*run
	order []string
	limit func() (heavy, light int)
	wake  chan struct{}
	// OnFinish runs after a run reaches a final state (set before Submit is used).
	OnFinish func(RunMeta)
	// SecretEnv turns RunSpec.Secrets into environment entries for that child only.
	SecretEnv func(names []string) []string
	closed    bool
	wg        sync.WaitGroup
}

func newManager(dataDir string, limit func() (heavy, light int)) (*Manager, error) {
	m := &Manager{dir: filepath.Join(dataDir, "runs"), runs: map[string]*run{}, limit: limit, wake: make(chan struct{}, 1)}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return nil, err
	}
	m.loadHistory()
	go m.scheduler() // the caller pokes once its hooks are set, so resumed runs get their secrets
	return m, nil
}

// Start schedules runs left queued by the previous studio process.
func (m *Manager) Start() { m.poke() }

func (m *Manager) loadHistory() {
	entries, _ := os.ReadDir(m.dir)
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(m.dir, e.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var meta RunMeta
		if json.Unmarshal(data, &meta) != nil || meta.ID == "" {
			continue
		}
		if meta.Status == "running" { // the previous studio process died while this ran
			if meta.Kind == "script" {
				resetForRetry(&meta) // scripts are idempotent: run again
			} else {
				meta.Status = "interrupted"
				if meta.Ended.IsZero() {
					meta.Ended = time.Now()
				}
			}
			_ = writeJSONAtomic(filepath.Join(m.dir, meta.ID, "meta.json"), meta, 0o644)
		}
		if meta.Status == "queued" { // still waiting: keep it in the queue
			m.runs[meta.ID] = &run{meta: meta, subs: map[chan Event]struct{}{}, done: make(chan struct{})}
			m.order = append(m.order, meta.ID)
			continue
		}
		m.runs[meta.ID] = &run{meta: meta, done: closedChan()}
		m.order = append(m.order, meta.ID)
	}
	sort.Slice(m.order, func(i, j int) bool { return m.runs[m.order[i]].meta.Created.Before(m.runs[m.order[j]].meta.Created) })
	// keep history bounded: drop the oldest runs beyond maxHistory
	for len(m.order) > maxHistory {
		id := m.order[0]
		m.order = m.order[1:]
		delete(m.runs, id)
		_ = os.RemoveAll(filepath.Join(m.dir, id))
	}
}

const maxHistory = 300

func closedChan() chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}

func newRunID() string {
	return time.Now().Format("060102-150405") + "-" + newToken()[:6]
}

// Submit validates the spec and queues it.
func (m *Manager) Submit(spec RunSpec) (RunMeta, error) {
	metas, err := m.SubmitChain([]RunSpec{spec}, nil)
	if err != nil {
		return RunMeta{}, err
	}
	return metas[0], nil
}

// SubmitChain queues several runs at once. deps[i] lists indexes (into specs) that run i
// waits for; independent runs start in parallel as capacity allows.
func (m *Manager) SubmitChain(specs []RunSpec, deps [][]int) ([]RunMeta, error) {
	for _, spec := range specs {
		if len(spec.Argv) == 0 {
			return nil, errors.New("empty command")
		}
		if spec.Cwd == "" {
			return nil, errors.New("missing working directory")
		}
		if st, err := os.Stat(spec.Cwd); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("working directory not found: %s", spec.Cwd)
		}
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, errors.New("studio is shutting down")
	}
	group := ""
	if len(specs) > 1 {
		group = "g-" + newRunID()
	}
	created := make([]*run, len(specs))
	for i, spec := range specs {
		if group != "" {
			spec.Group = group
		}
		if i < len(deps) {
			for _, d := range deps[i] {
				if d < 0 || d >= i {
					m.mu.Unlock()
					return nil, fmt.Errorf("run %d may only wait for earlier runs", i)
				}
				spec.After = append(spec.After, created[d].meta.ID)
			}
		}
		r := &run{meta: RunMeta{RunSpec: spec, ID: newRunID(), Status: "queued", Created: time.Now()}, subs: map[chan Event]struct{}{}, done: make(chan struct{})}
		if err := os.MkdirAll(filepath.Join(m.dir, r.meta.ID), 0o755); err != nil {
			m.mu.Unlock()
			return nil, err
		}
		r.persist(m.dir)
		title := "Đang chờ tới lượt"
		if len(r.meta.After) > 0 {
			title = "Chờ bước trước xong"
		}
		r.emit(m.dir, Event{Kind: "status", Title: title})
		m.runs[r.meta.ID] = r
		m.order = append(m.order, r.meta.ID)
		created[i] = r
	}
	m.mu.Unlock()
	out := make([]RunMeta, len(created))
	for i, r := range created {
		out[i] = r.snapshot()
	}
	m.poke()
	return out, nil
}

// Group returns the runs of one chain in submission order.
func (m *Manager) Group(group string) []RunMeta {
	out := []RunMeta{}
	if group == "" {
		return out
	}
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	for _, id := range ids {
		if r, ok := m.Get(id); ok {
			if meta := r.snapshot(); meta.Group == group {
				meta.Prompt = ""
				out = append(out, meta)
			}
		}
	}
	return out
}

func (m *Manager) poke() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) scheduler() {
	for range m.wake {
		for m.startNext() {
		}
	}
}

// startNext launches one runnable queued run, or fails one whose prerequisite failed.
// It reports whether it changed anything, so the caller loops until nothing is ready.
func (m *Manager) startNext() bool {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return false
	}
	status := map[string]string{}
	heavyRunning, lightRunning := 0, 0
	for _, id := range m.order {
		r := m.runs[id]
		r.mu.Lock()
		status[id] = r.meta.Status
		if r.meta.Status == "running" {
			if r.meta.Heavy {
				heavyRunning++
			} else {
				lightRunning++
			}
		}
		r.mu.Unlock()
	}
	heavyCap, lightCap := m.limit()
	for _, id := range m.order {
		r := m.runs[id]
		r.mu.Lock()
		if r.meta.Status != "queued" {
			r.mu.Unlock()
			continue
		}
		ready, blockedBy := true, ""
		for _, dep := range r.meta.After {
			switch status[dep] {
			case "succeeded":
			case "failed", "stopped", "interrupted", "":
				blockedBy = dep
			default:
				ready = false
			}
		}
		if blockedBy != "" {
			r.meta.Status = "failed"
			r.meta.Error = "bước trước không hoàn thành: " + blockedBy
			r.meta.Ended = time.Now()
			r.mu.Unlock()
			m.mu.Unlock()
			r.emit(m.dir, Event{Kind: "status", Title: "Bỏ qua vì bước trước không hoàn thành", Error: true})
			r.persist(m.dir)
			m.closeRun(r)
			return true
		}
		capOK := (r.meta.Heavy && heavyRunning < heavyCap) || (!r.meta.Heavy && lightRunning < lightCap)
		if !ready || !capOK {
			r.mu.Unlock()
			continue
		}
		r.meta.Status = "running"
		r.meta.Started = time.Now()
		r.mu.Unlock()
		m.wg.Add(1)
		m.mu.Unlock()
		go func() {
			defer m.wg.Done()
			m.execute(r)
			m.poke()
		}()
		return true
	}
	m.mu.Unlock()
	return false
}

// closeRun ends a run that never started: release waiters and live subscribers.
func (m *Manager) closeRun(r *run) {
	if m.OnFinish != nil {
		m.OnFinish(r.snapshot())
	}
	r.mu.Lock()
	for ch := range r.subs {
		close(ch)
	}
	r.subs = map[chan Event]struct{}{}
	r.mu.Unlock()
	close(r.done)
}

func (m *Manager) execute(r *run) {
	defer close(r.done)
	spec := r.meta.RunSpec
	if spec.FingerprintDir != "" { // fingerprint the inputs this run actually starts from
		h := videoInputsHash(spec.FingerprintDir)
		r.mu.Lock()
		r.meta.InputsHash = h
		r.mu.Unlock()
	}
	cmd := exec.Command(spec.Argv[0], spec.Argv[1:]...)
	cmd.Dir = spec.Cwd
	cmd.Env = childEnv()
	if len(spec.Secrets) > 0 && m.SecretEnv != nil {
		cmd.Env = append(cmd.Env, m.SecretEnv(spec.Secrets)...)
	}
	cmd.Stdin = nil // agents must not wait for terminal input
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// A detached grandchild can keep our pipes open after the main process exits;
	// WaitDelay makes Wait give up on them instead of hanging the queue.
	cmd.WaitDelay = 3 * time.Second
	logFile, _ := os.OpenFile(filepath.Join(m.dir, r.meta.ID, "stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	outR, outW := io.Pipe()
	errR, errW := io.Pipe()
	if logFile != nil {
		cmd.Stdout = io.MultiWriter(outW, logFile)
	} else {
		cmd.Stdout = outW
	}
	cmd.Stderr = errW
	r.mu.Lock()
	r.logFile = logFile
	r.mu.Unlock()
	if err := cmd.Start(); err != nil {
		outW.Close()
		errW.Close()
		m.finish(r, -1, err)
		return
	}
	r.mu.Lock()
	r.cmd = cmd
	r.meta.PID = cmd.Process.Pid
	stoppedEarly := r.stopped
	r.mu.Unlock()
	if stoppedEarly {
		m.terminate(r, cmd.Process.Pid)
	}
	r.persist(m.dir)
	r.emit(m.dir, Event{Kind: "status", Title: "Đang chạy", Body: fmt.Sprintf("PID %d · %s", cmd.Process.Pid, shortCmd(spec.Argv))})
	limit := runTimeout(spec)
	timer := time.AfterFunc(limit, func() {
		r.mu.Lock()
		r.timedOut, r.stopped = true, true
		r.mu.Unlock()
		r.emit(m.dir, Event{Kind: "status", Title: "Quá giới hạn thời gian, đang dừng", Body: limit.String(), Error: true})
		m.terminate(r, cmd.Process.Pid)
	})
	defer timer.Stop()

	parser := newParser(spec.Kind, spec.Agent)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for sc.Scan() {
			line := sc.Text()
			r.mu.Lock()
			usage := r.meta.Usage
			r.mu.Unlock()
			parser.Line(line, func(e Event) { r.emit(m.dir, e) }, &usage)
			r.mu.Lock()
			r.meta.Usage = usage
			r.mu.Unlock()
		}
		if err := sc.Err(); err != nil {
			r.emit(m.dir, Event{Kind: "error", Body: "đầu ra không đọc được tiếp: " + err.Error(), Error: true})
		}
		_, _ = io.Copy(io.Discard, outR) // keep the child unblocked after a scanner error
	}()
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(errR)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if strings.TrimSpace(line) == "" || noisyStderr(line) {
				continue
			}
			kind := "stderr"
			if spec.Kind == "script" {
				kind = "log" // scripts report progress on stderr
			}
			r.emit(m.dir, Event{Kind: kind, Body: clip(stripANSI(line))})
		}
		_, _ = io.Copy(io.Discard, errR)
	}()
	err := cmd.Wait()
	outW.Close()
	errW.Close()
	wg.Wait()
	code := 0
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) || errors.Is(err, exec.ErrWaitDelay) {
		err = nil // the exit code carries the outcome; leftover pipes are not a failure
	}
	if err != nil {
		code = -1
	}
	m.finish(r, code, err)
}

// terminate sends SIGTERM to the run's process group and SIGKILL if it is still alive later.
func (m *Manager) terminate(r *run, pid int) {
	killGroup(pid, syscall.SIGTERM)
	go func() {
		select {
		case <-r.done:
		case <-time.After(6 * time.Second):
			killGroup(pid, syscall.SIGKILL)
		}
	}()
}

func (m *Manager) finish(r *run, code int, err error) {
	r.mu.Lock()
	r.meta.Ended = time.Now()
	r.meta.ExitCode = &code
	switch {
	case r.requeue:
		resetForRetry(&r.meta)
	case r.interrupted:
		r.meta.Status = "interrupted"
	case r.timedOut:
		r.meta.Status = "failed"
		r.meta.Error = fmt.Sprintf("quá giới hạn %s", runTimeout(r.meta.RunSpec))
	case r.stopped:
		r.meta.Status = "stopped"
	case err != nil:
		r.meta.Status = "failed"
		r.meta.Error = err.Error()
	case code == 0:
		r.meta.Status = "succeeded"
	default:
		r.meta.Status = "failed"
	}
	if r.logFile != nil {
		r.logFile.Close()
		r.logFile = nil
	}
	status, elapsed := r.meta.Status, r.meta.Ended.Sub(r.meta.Started).Round(100*time.Millisecond)
	r.mu.Unlock()
	body := fmt.Sprintf("mã thoát %d · %s", code, elapsed)
	if err != nil {
		body = err.Error()
	}
	r.emit(m.dir, Event{Kind: "status", Title: statusLabel(status), Body: body, Error: status == "failed"})
	r.persist(m.dir)
	if m.OnFinish != nil {
		m.OnFinish(r.snapshot())
	}
	r.mu.Lock()
	for ch := range r.subs {
		close(ch)
	}
	r.subs = map[chan Event]struct{}{}
	r.events = nil // history is on disk; free memory for long agent runs
	r.mu.Unlock()
}

func statusLabel(s string) string {
	return map[string]string{"succeeded": "Hoàn thành", "failed": "Thất bại", "stopped": "Đã dừng", "interrupted": "Bị gián đoạn", "running": "Đang chạy", "queued": "Đang chờ"}[s]
}

func (r *run) emit(dir string, e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e.Seq = r.meta.Events + 1
	if e.TS == 0 {
		e.TS = time.Now().UnixMilli()
	}
	r.meta.Events = e.Seq
	r.events = append(r.events, e)
	if f, err := os.OpenFile(filepath.Join(dir, r.meta.ID, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		b, _ := json.Marshal(e)
		f.Write(append(b, '\n'))
		f.Close()
	}
	for ch := range r.subs {
		select {
		case ch <- e:
		default:
			delete(r.subs, ch)
			close(ch)
		}
	}
}

func (r *run) persist(dir string) {
	r.mu.Lock()
	meta := r.meta
	r.mu.Unlock()
	_ = writeJSONAtomic(filepath.Join(dir, meta.ID, "meta.json"), meta, 0o644)
}

func (r *run) snapshot() RunMeta {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.meta
}

// List returns runs newest first.
func (m *Manager) List(limit int) []RunMeta {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	out := []RunMeta{}
	for i := len(ids) - 1; i >= 0 && len(out) < limit; i-- {
		m.mu.Lock()
		r := m.runs[ids[i]]
		m.mu.Unlock()
		out = append(out, r.snapshot())
	}
	return out
}

func (m *Manager) Get(id string) (*run, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	return r, ok
}

// Events returns stored events after seq; history comes from disk when not in memory.
func (m *Manager) Events(r *run, after int) []Event {
	r.mu.Lock()
	if len(r.events) > 0 || r.meta.Events == 0 {
		out := []Event{}
		for _, e := range r.events {
			if e.Seq > after {
				out = append(out, e)
			}
		}
		r.mu.Unlock()
		return out
	}
	id := r.meta.ID
	r.mu.Unlock()
	f, err := os.Open(filepath.Join(m.dir, id, "events.jsonl"))
	if err != nil {
		return nil
	}
	defer f.Close()
	out := []Event{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Seq > after {
			out = append(out, e)
		}
	}
	return out
}

// Subscribe returns backlog after seq plus a channel for live events (nil when finished).
func (m *Manager) Subscribe(r *run, after int) ([]Event, chan Event, func()) {
	r.mu.Lock()
	if !r.meta.active() {
		r.mu.Unlock()
		return m.Events(r, after), nil, func() {}
	}
	// copy the backlog and register under one lock so no event falls between them
	backlog := []Event{}
	for _, e := range r.events {
		if e.Seq > after {
			backlog = append(backlog, e)
		}
	}
	ch := make(chan Event, 256)
	r.subs[ch] = struct{}{}
	r.mu.Unlock()
	return backlog, ch, func() {
		r.mu.Lock()
		if _, ok := r.subs[ch]; ok {
			delete(r.subs, ch)
			close(ch)
		}
		r.mu.Unlock()
	}
}

// Stop terminates the run's whole process group: SIGTERM, then SIGKILL after a grace period.
func (m *Manager) Stop(id string) error {
	r, ok := m.Get(id)
	if !ok {
		return errors.New("run not found")
	}
	r.mu.Lock()
	if !r.meta.active() {
		r.mu.Unlock()
		return nil
	}
	r.stopped = true
	cmd := r.cmd
	queued := r.meta.Status == "queued"
	if queued {
		r.meta.Status = "stopped"
		r.meta.Ended = time.Now()
	}
	r.mu.Unlock()
	if queued {
		r.emit(m.dir, Event{Kind: "status", Title: "Đã huỷ trước khi chạy"})
		r.persist(m.dir)
		m.closeRun(r)
		m.poke() // runs waiting on this one now fail instead of waiting forever
		return nil
	}
	if cmd != nil && cmd.Process != nil {
		m.terminate(r, cmd.Process.Pid)
	} // otherwise execute sees r.stopped right after Start and terminates it there
	return nil
}

func killGroup(pid int, sig syscall.Signal) {
	if pid <= 0 {
		return
	}
	if err := syscall.Kill(-pid, sig); err != nil {
		_ = syscall.Kill(pid, sig)
	}
}

// Shutdown stops running work and waits briefly for it to exit. Queued runs stay queued
// on disk, and running scripts are queued again, so the next start carries on.
func (m *Manager) Shutdown(timeout time.Duration) {
	m.mu.Lock()
	m.closed = true
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	for _, id := range ids {
		r, ok := m.Get(id)
		if !ok {
			continue
		}
		r.mu.Lock()
		running := r.meta.Status == "running"
		if running {
			r.requeue = r.meta.Kind == "script"
			r.interrupted = !r.requeue
		}
		r.mu.Unlock()
		if running {
			_ = m.Stop(id)
		}
	}
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// Busy reports whether any run, including a free agent run, is queued or running for a project.
func (m *Manager) Busy(project string) bool {
	for _, meta := range m.List(200) {
		if meta.Project == project && meta.active() {
			return true
		}
	}
	return false
}

// ActiveFor reports the active run for a project step, if any.
func (m *Manager) ActiveFor(project string) map[string]string {
	out := map[string]string{}
	for _, meta := range m.List(200) {
		if meta.Project == project && meta.active() && meta.Step != "" {
			out[meta.Step] = meta.ID
		}
	}
	return out
}

func childEnv() []string {
	env := []string{}
	for _, kv := range os.Environ() {
		// a studio started from inside an agent session must not make children think they are nested
		if strings.HasPrefix(kv, "CLAUDECODE=") || strings.HasPrefix(kv, "CLAUDE_CODE_ENTRYPOINT=") {
			continue
		}
		// image keys reach only the image run, through SecretEnv; never every agent
		if strings.HasPrefix(kv, "QH_IMAGE_") || strings.HasPrefix(kv, "AIHUB_API_KEY=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "NO_COLOR=1", "PYTHONUNBUFFERED=1")
}

func shortCmd(argv []string) string {
	parts := []string{}
	for i, a := range argv {
		if len(a) > 80 {
			a = a[:77] + "…"
		}
		if i > 6 {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

var progressBar = regexp.MustCompile(`\d+%\|`)

// noisyStderr drops progress bars (tqdm prints dozens per second) and known harmless MCP noise.
func noisyStderr(line string) bool {
	l := strings.ToLower(line)
	return strings.Contains(l, "rmcp::transport") || progressBar.MatchString(line)
}

// resetForRetry puts a run back in the queue as if it had not started.
func resetForRetry(meta *RunMeta) {
	meta.Status = "queued"
	meta.Started, meta.Ended = time.Time{}, time.Time{}
	meta.PID, meta.ExitCode, meta.Error = 0, nil, ""
}

// runTimeout bounds a run so a hung job cannot hold its slot forever.
func runTimeout(spec RunSpec) time.Duration {
	if spec.TimeoutSec > 0 {
		return time.Duration(spec.TimeoutSec) * time.Second
	}
	switch {
	case spec.Kind == "agent":
		return 90 * time.Minute
	case spec.Step == "render":
		return 90 * time.Minute
	case spec.Heavy:
		return 45 * time.Minute
	default:
		return 30 * time.Minute
	}
}
