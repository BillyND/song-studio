# Brainstorm: multi-turn agent chat panel

Status: accepted 2026-10-24 (user picked the recommended option on all three questions).

## Outcome
- A global agent chat panel docked on the right, resizable, opened from any page by a button or ⌘J.
- The panel knows the current song and shows it as a removable context chip ("📁 Bài: …"). Without a song, the agent works in the songs root.
- Multi-turn conversations. Each turn renders with the existing run timeline (text, thinking, tools, result).
- A list of past conversations, which can be reopened and continued after an app restart.
- Every finished agent run from a step button gets a "Trả lời · Reply" button that opens the chat on that run's session.
- Chat prompts tell the agent to ask and stop when something is unclear, instead of guessing.

## Decisions
- **Turn model:** one process per message, resuming the CLI session.
  - Commands: `claude -p --resume <id>`, `codex exec resume <id> <msg>`, `omp -p --resume <id>`.
  - The session id is already captured in `Usage.Session` (`parsers.go`).
  - This reuses the run queue, parsers, stop and persistence.
- **Sending while a turn runs:** Enter queues the message, shown in a tray above the input and sent automatically when the turn ends. A secondary "Dừng & gửi · Stop & send" button stops the turn and sends right away.
- **Step buttons:** they stay as they are and gain "Trả lời · Reply". They do not become chat messages.

## Constraints
- Each conversation keeps a fixed cwd, because claude looks up sessions by project directory.
- Keep the internal names: bundle id, config path, data path and keychain service.
- The UI is bilingual through `t()`/`tr()`.
- Chat turns are light runs, so they do not take a heavy slot.
- Secrets never reach prompts or logs.
- Run pruning must not drop runs that a kept conversation still references.

## Non-goals
- Switching agents inside a conversation; a new agent starts a new conversation.
- File or image attachments.
- Undo for agent edits.
- Mid-turn steering through a long-lived process: claude `--input-format stream-json`, codex app-server and omp rpc each use a different protocol and would not survive a restart.

## Trade-offs
| Approach | Load-bearing assumption | Fails first when |
|---|---|---|
| Per-turn resume (chosen) | Every CLI resumes a session by id from the same cwd | A CLI changes its resume flags or scopes sessions differently, for example omp |
| Long-lived process with streamed input | Each CLI keeps a stable bidirectional protocol | The app restarts or quits mid-conversation, or a CLI updates its protocol |

Better approaches: none. Per-turn resume is the requested chat behaviour at the lowest cost, because session ids are already captured.

## Acceptance criteria
- In the real app, with each of claude, codex and omp: ask a question, get a reply, ask a follow-up, and the agent remembers the context.
- A message sent during a running turn is queued, then sent automatically. "Stop & send" interrupts the turn and sends.
- Conversations survive an app restart and can be continued.
- "Reply" on a finished lyrics-review run continues that exact session.
- `go test -race` passes. A headless English scan finds no Vietnamese UI strings.

## Risks to verify first
- How omp `--resume` behaves with `-p --mode json` and a given cwd.
- Whether codex `exec resume` accepts `--json` and the sandbox flags.
- Whether a resumed claude turn emits a new `session_id`. If it does, keep the latest id.
