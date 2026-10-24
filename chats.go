package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Chat is a multi-turn conversation with one coding agent. Each message the user sends becomes
// an agent run that resumes the CLI session of the previous turn, so the agent keeps the context.
type Chat struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Agent   string `json:"agent"`
	Model   string `json:"model,omitempty"`
	Target  string `json:"target"` // project (a song folder), skill, or songs (the folder of all songs)
	Project string `json:"project,omitempty"`
	Cwd     string `json:"cwd"`
	// Session is the CLI session the next turn resumes; empty until the first turn reports one.
	Session string `json:"session,omitempty"`
	// Primed is set once the agent has been told it is in a chat and may ask questions back.
	Primed bool       `json:"primed,omitempty"`
	Turns  []ChatTurn `json:"turns"`
	// Queue holds messages sent while a turn was running; they go out together when it ends.
	Queue []string `json:"queue"`
	// Error says why the queue could not go out on its own (when a turn ended or at startup).
	Error   string    `json:"error,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// ChatTurn pairs what the user wrote with the run that answered it. A chat continued from a
// step's agent run starts with that run and no text.
type ChatTurn struct {
	Run  string `json:"run"`
	Text string `json:"text,omitempty"`
	// Status is filled in views only: the run's status, or "" when it is no longer in the history.
	Status string `json:"status,omitempty"`
}

// ChatView is a chat with the state of its latest turn, for the UI.
type ChatView struct {
	Chat
	Status string `json:"status,omitempty"` // status of the latest turn's run, "" when it is gone
}

var chatTargets = []string{"project", "skill", "songs"}

type ChatStore struct {
	mu       sync.Mutex
	dir      string
	chats    map[string]*Chat
	cfg      *ConfigStore
	runs     *Manager
	projects Projects
}

func newChatStore(dataDir string, cfg *ConfigStore, runs *Manager, projects Projects) (*ChatStore, error) {
	s := &ChatStore{dir: filepath.Join(dataDir, "chats"), chats: map[string]*Chat{}, cfg: cfg, runs: runs, projects: projects}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	for _, c := range loadChats(dataDir) {
		s.chats[c.ID] = c
	}
	return s, nil
}

func loadChats(dataDir string) []*Chat {
	dir := filepath.Join(dataDir, "chats")
	entries, _ := os.ReadDir(dir)
	out := []*Chat{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var c Chat
		if json.Unmarshal(data, &c) == nil && c.ID != "" {
			out = append(out, &c)
		}
	}
	return out
}

// chatRunIDs lists the runs saved chats point at, so pruning old run history keeps them.
func chatRunIDs(dataDir string) map[string]bool {
	out := map[string]bool{}
	for _, c := range loadChats(dataDir) {
		for _, t := range c.Turns {
			out[t.Run] = true
		}
	}
	return out
}

func (s *ChatStore) save(c *Chat) error {
	c.Updated = time.Now()
	return writeJSONAtomic(filepath.Join(s.dir, c.ID+".json"), c, 0o644)
}

func (s *ChatStore) view(c *Chat) ChatView {
	v := ChatView{Chat: *c}
	v.Turns = make([]ChatTurn, len(c.Turns))
	for i, t := range c.Turns {
		t.Status = ""
		if r, ok := s.runs.Get(t.Run); ok {
			t.Status = r.snapshot().Status
		}
		v.Turns[i] = t
	}
	v.Queue = append([]string{}, c.Queue...)
	if n := len(v.Turns); n > 0 {
		v.Status = v.Turns[n-1].Status
	}
	return v
}

// List returns chats, most recently used first.
func (s *ChatStore) List() []ChatView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ChatView, 0, len(s.chats))
	for _, c := range s.chats {
		out = append(out, s.view(c))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out
}

func (s *ChatStore) Get(id string) (ChatView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.chats[id]
	if !ok {
		return ChatView{}, os.ErrNotExist
	}
	return s.view(c), nil
}

// ChatRequest starts a chat: about a song, the skill or all songs, or continuing a step's run.
type ChatRequest struct {
	Agent   string `json:"agent"`
	Model   string `json:"model"`
	Target  string `json:"target"`
	Project string `json:"project"`
	Text    string `json:"text"`
	FromRun string `json:"fromRun"`
}

// Create starts a chat and sends its first message, if any. Continuing a run that a chat
// already continues returns that chat.
func (s *ChatStore) Create(req ChatRequest) (ChatView, error) {
	cfg := s.cfg.Get()
	c := &Chat{ID: "c-" + newRunID(), Created: time.Now(), Turns: []ChatTurn{}, Queue: []string{}}
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.FromRun != "" {
		r, ok := s.runs.Get(req.FromRun)
		if !ok {
			return ChatView{}, os.ErrNotExist
		}
		meta := r.snapshot()
		// checked and added under one lock, so a double click cannot make two chats on one session
		for _, old := range s.chats {
			for _, t := range old.Turns {
				if t.Run == meta.ID {
					return s.view(old), nil
				}
			}
		}
		switch {
		case meta.Kind != "agent":
			return ChatView{}, errors.New(tr("chỉ trả lời được lượt chạy của agent", "only an agent run can be replied to"))
		case meta.active():
			return ChatView{}, errors.New(tr("agent chưa làm xong lượt này", "the agent has not finished this run yet"))
		case meta.Usage.Session == "":
			return ChatView{}, errors.New(tr("lượt này không có phiên agent để nói tiếp", "this run has no agent session to continue"))
		}
		c.Agent, c.Model, c.Cwd, c.Project, c.Session = meta.Agent, meta.Model, meta.Cwd, meta.Project, meta.Usage.Session
		c.Title, c.Target = meta.Title, "songs"
		if meta.Project != "" {
			c.Target = "project"
		} else if meta.Step == "skill" {
			c.Target = "skill"
		}
		c.Turns = append(c.Turns, ChatTurn{Run: meta.ID})
	} else {
		c.Agent, c.Model, c.Target = req.Agent, strings.TrimSpace(req.Model), req.Target
		if c.Agent == "" {
			c.Agent = cfg.DefaultAgent
		}
		switch c.Target {
		case "project":
			dir, err := s.projects.dir(req.Project)
			if err != nil {
				return ChatView{}, err
			}
			c.Cwd, c.Project = dir, req.Project
		case "skill":
			c.Cwd = cfg.SkillDir
		case "songs":
			c.Cwd = cfg.ProjectsDir
		default:
			return ChatView{}, errors.New(tr("chat phải về một bài, skill hoặc mọi bài", "a chat is about a song, the skill or all songs"))
		}
		if st, err := os.Stat(c.Cwd); err != nil || !st.IsDir() {
			return ChatView{}, fmt.Errorf(tr("không thấy thư mục làm việc: %s", "working directory not found: %s"), c.Cwd)
		}
		c.Title = firstLine(req.Text, 60)
		c.Primed = true // the header of its first turn says it is a chat
	}
	// an agent that cannot run here (unknown, or omp without Full access) fails now, before a
	// chat is saved that could never send anything
	if _, err := agentResumeArgv(cfg, c.Agent, c.Model, "", c.Cwd, c.Session); err != nil {
		return ChatView{}, err
	}
	if c.Title == "" {
		c.Title = tr("Cuộc trò chuyện mới", "New chat")
	}
	if text := strings.TrimSpace(req.Text); text != "" && req.FromRun == "" {
		c.Queue = append(c.Queue, text)
		sent, err := s.drain(c)
		if sent { // its turn is running: keep the chat even if saving it failed
			s.chats[c.ID] = c
		}
		if err != nil {
			return ChatView{}, err
		}
	} else if err := s.save(c); err != nil {
		return ChatView{}, err
	}
	s.chats[c.ID] = c
	return s.view(c), nil
}

// Send adds a message. With no turn running it goes out now; otherwise it waits in the queue,
// and now stops the running turn so the queue goes out as soon as that turn ends.
func (s *ChatStore) Send(id, text string, now bool) (ChatView, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return ChatView{}, errors.New(tr("tin nhắn trống", "the message is empty"))
	}
	v, running, err := s.enqueue(id, text)
	if err != nil || running == "" || !now {
		return v, err
	}
	// outside the lock: stopping a run that has not started finishes it right here, and
	// runFinished takes the lock to send the queue
	if err := s.runs.Stop(running); err != nil {
		return v, err
	}
	return s.Get(id)
}

// enqueue adds a message and sends the queue when no turn is running. It returns the turn
// still running, if any.
func (s *ChatStore) enqueue(id, text string) (ChatView, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.chats[id]
	if !ok {
		return ChatView{}, "", os.ErrNotExist
	}
	c.Queue = append(c.Queue, text)
	running := s.runningTurn(c)
	if running != "" {
		return s.view(c), running, s.save(c)
	}
	sent, err := s.drain(c)
	if err != nil && !sent {
		c.Queue = c.Queue[:len(c.Queue)-1] // not sent: the user sees the error and can send it again
	}
	return s.view(c), "", err
}

// RemoveQueued drops one waiting message.
func (s *ChatStore) RemoveQueued(id string, i int) (ChatView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.chats[id]
	if !ok {
		return ChatView{}, os.ErrNotExist
	}
	if i < 0 || i >= len(c.Queue) {
		return ChatView{}, os.ErrNotExist
	}
	c.Queue = append(c.Queue[:i], c.Queue[i+1:]...)
	if len(c.Queue) == 0 {
		c.Error = ""
	}
	if err := s.save(c); err != nil {
		return ChatView{}, err
	}
	return s.view(c), nil
}

// Delete stops a running turn and forgets the chat; its runs stay in the run history.
func (s *ChatStore) Delete(id string) error {
	s.mu.Lock()
	c, ok := s.chats[id]
	if !ok {
		s.mu.Unlock()
		return os.ErrNotExist
	}
	delete(s.chats, id)
	running := s.runningTurn(c)
	err := os.Remove(filepath.Join(s.dir, id+".json"))
	s.mu.Unlock()
	if running != "" {
		_ = s.runs.Stop(running)
	}
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return err
}

// runFinished records the session a turn reported and sends what was queued during it.
func (s *ChatStore) runFinished(meta RunMeta) {
	if meta.Chat == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.chats[meta.Chat]
	if !ok {
		return
	}
	if meta.Usage.Session != "" {
		c.Session = meta.Usage.Session
	}
	// the studio is quitting (the queue goes out on the next start), or a message sent just as
	// this run ended has already started the next turn
	if meta.Status == "interrupted" || s.runningTurn(c) != "" {
		_ = s.save(c)
		return
	}
	s.sendQueued(c)
}

// Start sends messages left queued when the studio last quit.
func (s *ChatStore) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.chats {
		if len(c.Queue) > 0 && s.runningTurn(c) == "" {
			s.sendQueued(c)
		}
	}
}

// sendQueued sends the queue with nobody waiting on the result, so a failure is kept on the
// chat for the panel to show. Callers hold s.mu.
func (s *ChatStore) sendQueued(c *Chat) {
	if _, err := s.drain(c); err != nil {
		c.Error = err.Error()
		_ = s.save(c)
	}
}

// runningTurn returns the latest turn's run while it is queued or running. Callers hold s.mu.
func (s *ChatStore) runningTurn(c *Chat) string {
	if n := len(c.Turns); n > 0 {
		if r, ok := s.runs.Get(c.Turns[n-1].Run); ok && r.snapshot().active() {
			return r.snapshot().ID
		}
	}
	return ""
}

// drain sends every queued message as one turn. Callers hold s.mu and know no turn is running.
// sent reports whether the turn was submitted, even when saving the chat afterwards failed.
func (s *ChatStore) drain(c *Chat) (sent bool, err error) {
	if len(c.Queue) == 0 {
		return false, nil
	}
	text := strings.Join(c.Queue, "\n\n")
	cfg := s.cfg.Get()
	if n := len(c.Turns); n > 0 {
		if r, ok := s.runs.Get(c.Turns[n-1].Run); ok {
			last := r.snapshot()
			// the previous turn may have ended a moment ago, before runFinished recorded its session
			if last.Usage.Session != "" {
				c.Session = last.Usage.Session
			}
			// a studio crash can leave that turn's agent alive; a second one on its session would
			// interleave with it
			if last.Status == "interrupted" && last.PID > 0 && syscall.Kill(-last.PID, 0) == nil {
				return false, fmt.Errorf(tr("agent của lượt trước (PID %d) vẫn đang chạy; dừng nó rồi gửi lại", "the previous turn's agent (PID %d) is still running; stop it, then send again"), last.PID)
			}
		}
	}
	prompt := text
	switch {
	case c.Session == "":
		// a new session (or one whose first turn never got going) needs the whole briefing
		prompt = chatHeader(cfg, c) + text
	case !c.Primed:
		prompt = tr("[Song Studio] Người dùng nhắn tiếp trong khung chat. Từ giờ bạn có thể hỏi lại: nếu chưa rõ hoặc cần người dùng chọn, hỏi ngắn gọn rồi dừng.\n\n",
			"[Song Studio] The user is following up in the chat panel. From now on you may ask back: if something is unclear or the user should choose, ask briefly and stop.\n\n") + text
	}
	argv, err := agentResumeArgv(cfg, c.Agent, c.Model, prompt, c.Cwd, c.Session)
	if err != nil {
		return false, err
	}
	meta, err := s.runs.Submit(RunSpec{Kind: "agent", Title: firstLine(text, 60), Project: c.Project, Cwd: c.Cwd,
		Agent: c.Agent, Model: c.Model, Prompt: prompt, Argv: argv, Chat: c.ID, Resume: c.Session})
	if err != nil {
		return false, err
	}
	c.Primed = true
	c.Queue = []string{}
	c.Error = ""
	c.Turns = append(c.Turns, ChatTurn{Run: meta.ID, Text: text})
	return true, s.save(c)
}

// chatHeader briefs the agent at the start of a chat session about where it works.
func chatHeader(cfg Config, c *Chat) string {
	var where string
	switch c.Target {
	case "project":
		where = fmt.Sprintf("Thư mục bài hát đang mở: %s. Đọc %s/SKILL.md và reference của bước liên quan trước khi sửa file của bài.", c.Cwd, cfg.SkillDir)
	case "skill":
		where = fmt.Sprintf("Bạn làm việc trên skill suno-que-huong ở %s. Đọc SKILL.md và các file liên quan trước khi sửa; giữ nguyên hành vi đã được chấp nhận, chỉ đổi đúng phần được nhờ, và chạy test trong scripts/tests nếu có.", c.Cwd)
	default:
		where = fmt.Sprintf("Thư mục chứa mọi bài hát: %s (mỗi bài một thư mục con có project.json). Skill suno-que-huong ở %s: đọc SKILL.md khi cần làm theo quy trình.", c.Cwd, cfg.SkillDir)
	}
	return "Bạn là agent trong khung chat của Song Studio (ứng dụng làm nhạc Suno và video), trò chuyện nhiều lượt với người dùng.\n" +
		where + "\n" +
		"Làm đúng việc được nhờ. Nếu yêu cầu chưa rõ hoặc cần người dùng chọn, hỏi ngắn gọn rồi dừng; người dùng trả lời ở tin nhắn sau.\n" +
		"Không in hay ghi khoá API ra bất kỳ đâu. Trả lời ngắn gọn, nói rõ đã làm gì và file nào đổi.\n" +
		tr("Trả lời bằng tiếng Việt.\n", "The studio is in English: reply in English. Song lyrics stay in their sung language.\n") +
		"\nTin nhắn của người dùng:\n"
}
