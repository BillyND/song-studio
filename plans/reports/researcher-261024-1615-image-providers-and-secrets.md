# Image providers, local generation, and API-key storage for Quê Hương Studio

Date: 2026-10-24. Scope: a local Mac app (Go server on 127.0.0.1, Preact UI, Python media scripts) that makes 15–25 16:9 scene images per karaoke song, plus a Settings page for provider keys.

Source tags: **[O]** means an official page I fetched today. **[S]** means a secondary source (a tracker, blog, or reseller). **[E]** means I measured it on this machine. **[X]** means my own extrapolation.

## Outcome (read this first)

1. **Keep a single "OpenAI Images-shaped" adapter as the backbone.** It covers OpenAI direct, the current aihub gateway, and OpenRouter. OpenRouter's `/api/v1/images` returns the same `data[0].b64_json` shape and serves Nano Banana, FLUX 3, Recraft and GPT Image behind one key [O]. Add a native Gemini adapter second, and a local mflux adapter third and optional. Do not adopt LiteLLM.
2. **Default model: `gpt-image-2.5-flare`, quality `high`, at `2048x1152` or `2560x1440`.** That costs about **$0.85–$1.10 for a 20-image song** [O, computed from OpenAI's own token formula]. The current request (`1536x1024`, 3:2) is the wrong aspect ratio. The renderer also upsamples it about 1.5×, because it scales sources to 1.22× the 1080p frame (`render-karaoke-video.py:52`). That makes today's images visibly soft.
3. **Best second provider: Google `gemini-nano-banana-2.1` at 16:9 2K (2752x1536).** It costs $0.0504 per image, about $1.01 per song, and Batch is half price. It accepts 4 character and 10 object reference images, which is the strongest set-consistency feature for its price [O].
4. **Store secrets in the macOS Keychain using `zalando/go-keyring`.** Inject the key only into the image-run child's `cmd.Env`, never into argv or the studio's own environment. The UI shows only the last 4 characters. Each provider gets a free "Test key" call. Be honest about the limit: a Keychain item created through `/usr/bin/security` can be read silently by any process running as you, including the coding agents the studio spawns. That is the same boundary as a 0600 file. What Keychain adds is encryption at rest and no plaintext file.
5. **Image-to-video: skip it.** The renderer already does slow zoom, pan and crossfades. API video costs about $0.10/s at 720p [O], and local video on an M1 Pro is impractical.
6. **Local generation is viable only as a free draft or offline mode.** This Mac is an **M1 Pro with 32 GB** [E]. FLUX.2 [klein] 4B (Apache-2.0) would take roughly 45–65 s per ~1 MP image [X], so 15–25 min per song. It also competes with the heavy render jobs.

---

## 1. Cloud image APIs

### 1a. Facts per provider (16:9 and 1080p-relevant)

| Provider / model | 16:9 output | Consistency tools | Price per image (relevant size) | Speed / limits | Notes |
|---|---|---|---|---|---|
| **OpenAI `gpt-image-2.5-flare`** (2026-09-08) [O] | Custom `WxH`: multiples of 16, ratio ≤3:1, 655,360–8,294,400 px. Above 2560x1440 is "experimental". `1920x1080` is **invalid**; `1920x1088` is valid. | Edits endpoint takes several reference images; input image tokens cost $8/M | 1920x1088: low $0.0044 · med $0.0103 · **high $0.0398** · xhigh $0.0707 · max $0.159. 2048x1152 high $0.0424. 2560x1440 high $0.0553 | Rated "Very fast". Up to 2 min for complex prompts. Tier 1 ("Build") limit is **20 images/min** | Quality tiers low→max. `moderation: auto\|low`. Org verification may be required. Not on the free tier |
| OpenAI `gpt-image-2.5-sunburst` [O] | Same | Best for editing precision | Same token rates; token counts can differ from Flare | Rated "Medium" speed | Use it for edits, not bulk scenes |
| OpenAI `gpt-image-2` [O] | Same | Always high-fidelity inputs | 1536x1024 high **$0.165**, med $0.041 | Same limits | Same per-token rate as 2.5 but **4× the tokens at high**. `gpt-image-1` was shut down on **2026-10-23**. `gpt-image-1.5`/`-1-mini` shut down 2026-12-01 |
| **Google `gemini-nano-banana-2.1`** (2026-10-06) [O] | Preset `16:9`: 1K = 1376x768, **2K = 2752x1536**, 4K = 5504x3072 | **Up to 4 character refs + 10 object refs** | 1K $0.0336 · **2K $0.0504** · 4K $0.113. **Batch is half price** | Not published | SynthID watermark on every image. Paid tier data is "not used to improve products". Uses the new `interactions` API |
| Google `gemini-3-pro-image` (Nano Banana Pro) [O] | Same presets | 5 character refs + **3 style refs** | 1K/2K $0.134 · 4K $0.24 | Not published | Premium; only worth it if 2.1 fails on consistency |
| Google Imagen 4 [O] | — | — | — | — | **Shut down 2026-08-17** in the Gemini API. Do not build for it |
| **BFL `flux-3-image`** [O] | `aspect_ratio: "16:9"` + `resolution` tier | Up to 10 refs, **free**; bounding boxes | 1K $0.048 · **1.5K (~2 MP) $0.07** · 2K (~4 MP) $0.10 · 4K $0.607 | Async: submit, then poll `polling_url` | Moderated outputs are not charged. Unknown fields return 422. Header is `x-key` |
| BFL `flux-3-image-klein` [O] | Same | Same | 1.5K $0.011 · 2K $0.014 | Same | Cheapest decent cloud option. Open weights under a "Kommunity" license |
| FLUX.2 [pro] via fal.ai [O] / Replicate [O] | Explicit W/H | Multi-reference | fal: $0.03 for the first MP + $0.015 per extra MP (1920x1080 ≈ **$0.045**). Replicate ≈ $0.015/MP | — | A mature resale path, but superseded by FLUX 3 |
| Ideogram 4.5 [S] | Yes | Character reference; style references | Low $0.03 · Med $0.06 · High $0.10 | 10 requests in flight by default [O] | Main strength is text rendering, which this app does not need. Commercial use is allowed [O FAQ] |
| Recraft V4.1 / V4 Styles [O] | Yes | **V4 Styles: 1–10 style refs** (+$0.005 per request) | V4.1 $0.035 (1K), Pro 2K $0.21. V4.1 Flash $0.007 | — | Leans toward illustration and vector work. Also on OpenRouter |
| Stability (Ultra / Core / SD3.5) [O] | Yes | Limited | Ultra $0.08 · Core $0.03 · SD3.5 Large $0.065 | — | Behind on quality; no reason to add it |
| **OpenRouter `/api/v1/images`** [O] | Normalised `aspect_ratio` + `resolution`, or explicit `size` | `input_references` (support varies by provider) | Provider price passed through, plus a **5.5% credit-purchase fee** [S] | Per provider | One key reaches `openai/gpt-image-2.5-*`, `google/gemini-nano-banana-2.1`, `black-forest-labs/flux-3-image(-klein)`, `recraft/*`. **No Ideogram or Stability** [O, live model list] |

### 1b. Cost for one song (20 images, output sized for this renderer)

The renderer needs about **2342x1318** source pixels to avoid upsampling (1.22× headroom over 1920x1080). Sizes are picked to land near that.

| Option | Size requested | Per image | **20 images** | Fit |
|---|---|---|---|---|
| OpenAI 2.5-flare high | 2560x1440 | $0.0553 | **$1.11** | Best quality-to-cost for an OpenAI-only setup |
| OpenAI 2.5-flare high | 2048x1152 | $0.0424 | **$0.85** | Recommended default (slight 1.14× upscale) |
| OpenAI 2.5-flare medium | 2560x1440 | $0.0143 | $0.29 | Draft or preview pass |
| Nano Banana 2.1 2K | 2752x1536 | $0.0504 | **$1.01** ($0.50 Batch) | Recommended second provider |
| FLUX 3 Image 2K | ~4 MP | $0.10 | $2.00 | Strong prompt adherence; references are free |
| FLUX 3 Klein 2K | ~4 MP | $0.014 | **$0.28** | Cheapest cloud option |
| Nano Banana Pro 2K | 2752x1536 | $0.134 | $2.68 | Only for hard consistency cases |
| gpt-image-2 high (legacy) | 1536x1024 | $0.165 | $3.30 | Avoid it: same quality tier at 4× the cost |
| Recraft V4.1 Pro 2K | ~2K | $0.21 | $4.20 | Not justified here |
| Local FLUX.2 klein 4B | ~1 MP, then upscale | $0 | $0 (15–25 min) | Draft or offline only |

OpenAI figures come from the formula in OpenAI's `GptImageTokenCalculator` bundle at $30/M output tokens. That formula reproduces the published gpt-image-2 table exactly (1536x1024 high → 5,488 tokens → $0.165). Prompt text tokens add about $0.001 per image. The aihub gateway's `gpt-image-2.5` is **not** an official OpenAI model ID; the official IDs are `-sunburst` and `-flare`. Its billing may differ.

### 1c. Ranking for this app

| Rank | Provider | Why |
|---|---|---|
| 1 | **OpenAI `gpt-image-2.5-flare`** (direct, or the existing gateway) | Exact pixel sizes, so 16:9 at any resolution with no resize step. Already wired up. Cheapest at this quality tier. 20 images/min is enough for 4 in parallel |
| 2 | **Google `gemini-nano-banana-2.1`** (native) | Similar price; Batch halves it. Best reference-image consistency for recurring characters. Different "look", which is useful as a fallback when OpenAI moderation blocks a lyric scene |
| 3 | **OpenRouter** (same adapter as #1) | One key for FLUX 3 / Klein, Recraft and Nano Banana without writing more adapters. Costs 5.5% extra and adds another party seeing your prompts |
| 4 | Local mflux | Free and private, but slow on an M1 Pro (see §2) |
| — | Ideogram, Stability, Recraft (direct) | No differentiator for text-free landscape and people scenes. Reach Recraft through OpenRouter if you ever need it |

**Title card:** the app draws the title itself, so the image needs **empty space and no text**, not good text rendering. Every model above will sometimes paint stray letters. Keep the existing "no text, no watermark" suffix and have the stills check catch it. Text rendering quality is therefore a low-weight criterion here.

**Consistency recipe (works with any provider):** generate the title or hero image first. Then pass it as a style reference to the other scenes: OpenAI `images/edits`, Gemini `input` images, FLUX 3 `images`, or Recraft V4 Styles. On OpenAI this adds input-image tokens at $8/M, roughly a few cents per image [X; measure from `usage`]. On FLUX 3 and Recraft Styles it is free or close to it.

## 2. Local generation on Apple Silicon

| Tool | Engine | Fit for this app | Health |
|---|---|---|---|
| **mflux** | MLX (native) | **Best.** A CLI or Python API that a Python script can call. Supports FLUX.2 klein, FLUX 3 Klein, Z-Image, Qwen-Image 2.1, and SeedVR2 for upscaling | MIT, 2.5k stars, v0.23.0 released 2026-10-16, very active [O] |
| Draw Things | Metal (own engine) | Good GUI. Its HTTP API (A1111-style, `:7860`) only works while the app is open with a model loaded [S] | Free app, active |
| ComfyUI | PyTorch MPS | Overkill. Heavy torch install, slower than MLX on Mac | Active |
| diffusers on MPS | PyTorch MPS | Slowest path. Large dependency set | Active |

**Speed:**

| Model (steps, size) | Measured | Source |
|---|---|---|
| FLUX.1 schnell (2 steps, 1024², including model load) | M1 Pro 32 GB ~160 s · M2 Pro ~54 s · M3 Pro ~80 s · M4 Pro ~34 s · M3 Max ~20 s · M4 Max ~19 s | mflux README benchmark table (older mflux, ballpark) [O] |
| FLUX.2 klein 4B (4 steps, 1024²) | M1 Max 64 GB: 21–32 s (mflux 0.17.5) | lilting.ch hands-on, 2026-04-30 [S] |
| FLUX.2 klein 4B on **this M1 Pro** | ~45–65 s per image [X: half the GPU cores and memory bandwidth of an M1 Max] | — |
| Z-Image Turbo (9 steps) | No Mac number found. Draw Things reports only 1.0–1.2× per-step gain over FLUX.1-dev | Draw Things blog [O] |

1344x768 is about 1.03 MP, so it takes about the same time as 1024². A 20-image song is therefore roughly **15–25 min** on this machine, and the result still needs a 1.75× upscale for this renderer.

**Licenses (checked live through the Hugging Face API today) [O]:**

| Model | License | Commercial use (e.g. a monetised YouTube channel) |
|---|---|---|
| FLUX.2 [klein] **4B** | Apache-2.0 | Yes |
| FLUX.1 [schnell] | Apache-2.0 | Yes |
| Z-Image-Turbo | Apache-2.0 | Yes |
| Qwen-Image | Apache-2.0 | Yes |
| FLUX.1 [dev], FLUX.2 [dev], FLUX.2 [klein] 9B | FLUX non-commercial | **No** without a paid BFL license |
| FLUX 3 Image Klein | "FLUX Kommunity License" (gated) | Unclear. One secondary source cites a $5M revenue cap [S]. Read it before use |

**Verdict:** if you want local mode, add it as an optional "Local (mflux, FLUX.2 klein 4B)" provider for free drafts or offline use. It should not be the default on an M1 Pro.

## 3. Image-to-video / Ken Burns (brief)

- The pipeline already has motion: "AI scene images with slow zoom and crossfades" (`references/karaoke-video.md:4`). The renderer alternates zoom and pan per scene.
- API image-to-video costs about **$0.10/s at 720p** (Gemini Omni Flash, token-billed) [O]. Veo 3.1 Preview in the Gemini API shut down on 2026-10-22 [O]. Animating 20 scenes for 4 s each would cost $8 or more per song, at **720p**, which is below the 1080p target.
- **Recommendation: do not add it.** A cheap upgrade later would be a 2.5D parallax effect from a depth map (mflux ships Apple Depth Pro). That needs no API and is outside this task's scope.

## 4. Secret storage for a local app

### 4a. Options

| Option | At rest | Who can read it silently | Rebuild friction | Verdict |
|---|---|---|---|---|
| File `~/.config/suno-que-huong/*.key`, 0600 (today) | Plaintext | Any process running as you; also backups and dotfile sync | None | Acceptable; it is the fallback |
| **Keychain via `zalando/go-keyring`** | Encrypted in the login keychain | Any process running as you that calls `/usr/bin/security`. The man page says the creating application, `security` itself, is trusted "without warning" [O] | None (pure Go, no cgo) | **Recommended** |
| Keychain via cgo (`keybase/go-keychain`, used by 99designs/keyring) with an ACL limited to the studio binary | Encrypted | Only the studio binary | **Prompts after every rebuild**, because an unsigned Go binary changes identity each build | Reject: bad UX. 99designs/keyring was last pushed 2024-05 [O] |
| Shell env var (`export OPENAI_API_KEY`) | In shell rc files | Every child process | — | Reject. `childEnv()` forwards the full environment to every coding agent |

`zalando/go-keyring` v0.2.8 is MIT-licensed and was pushed 2026-10-16 [O]. Its `Set` runs `security -i` and writes `add-generic-password … -w <secret>` to **stdin**, so the secret never appears in argv. `Get` passes only the service and account in argv; the secret comes back on stdout [O, source read].

### 4b. Passing the key to Python

- **Do:** set it in `cmd.Env` for the image run only, e.g. `append(childEnv(), "QH_IMAGE_API_KEY="+key)`. Read it in Python with `os.environ`. Never call `os.Setenv` in the studio process. `childEnv()` (runs.go:686) copies `os.Environ()` to *every* child, including the agents.
- **Don't:** put a key in argv. `ps` shows argv to every user, and `shortCmd()` writes argv into run metadata and the UI. Don't put a key in a URL either: for Gemini use the `x-goog-api-key` header, not `?key=`, because URLs end up in exceptions and proxy logs.
- On this machine (macOS 26.5.1), `ps eww` did not show a sibling process's environment, while argv was visible [E, run inside the agent sandbox; treat as indicative].

### 4c. Common leaks and how to close them

| Leak | Mitigation |
|---|---|
| Run logs (stdout/stderr are captured and streamed to the UI) | Python prints only the status code and the provider's error `message`. In Go, wrap the run log writer with a redactor that replaces the active key string with `••••last4` as defence in depth. Some gateways echo part of the key in 401 errors |
| `GET /api/bootstrap` or `PUT /api/settings` returning config | Keys never go into `config.json`. Store only non-secret metadata there (`last4`, `addedAt`, `lastCheck`) |
| Browser | The key is `POST`ed once to a dedicated endpoint (already behind the `X-Studio-Token`, Origin and Host guards in `security.go`). Use `type="password"` and `autocomplete="off"`, clear the field after saving, and never send the key back to the page |
| Shell history | Never document `security add-generic-password -w KEY`. Paste in the UI, or put `-w` last so `security` prompts for it |
| Tracebacks and crash logs | Default Python tracebacks print source lines, not values. Don't enable rich or `locals` tracebacks. Go panics don't print string contents |
| Skill folder | The studio can edit skill files (`PUT /api/skill/file`), so never store a key under `~/.claude/skills/...`. That is already true today |

### 4d. Free "Test key" calls (no generation charge)

| Provider | Call | Success signal |
|---|---|---|
| OpenAI / OpenAI-compatible gateway | `GET {base}/models/{model}`, falling back to `GET {base}/models` | 200 means the key is valid. 404 on the model means it is not available to this account (it may need org verification) |
| OpenRouter | `GET https://openrouter.ai/api/v1/key` | 200 with usage and limit (returns 401 without a key [E]) |
| Gemini | `GET https://generativelanguage.googleapis.com/v1beta/models` with `x-goog-api-key` | 200 |
| BFL | `GET https://api.bfl.ai/v1/credits` with `x-key` | 200 with credit balance [O] |
| Recraft | `GET https://external.api.recraft.ai/v1/users/me` | 200 with credits [O] |
| Stability | `GET https://api.stability.ai/v1/user/balance` | 200 (returns 401 without a key [E]) |
| Ideogram | No free endpoint found | Unverified; skip it, consistent with the ranking |

OpenAI org verification can only be proven by a real generation. Offer an optional "Tạo ảnh thử (~$0.006)" ("Test image") button that runs `1024x1024` at `low`.

### 4e. Rotation and removal

- **Replace:** overwrite the Keychain item (`-U`), update `last4` and `addedAt`, and run the test automatically.
- **Remove:** delete the Keychain item and its metadata. Runs then fail fast with "chưa có khoá" ("no key yet").
- **On a 401 during a run:** mark the provider `invalid` in metadata and show a banner on the Settings page. Do not retry.
- **Migration:** if `~/.config/suno-que-huong/aihub.key` exists, show "Nhập khoá cũ vào Keychain" ("Import old key into Keychain"). After a successful import, ask before deleting the file. Agents run the skill script standalone, outside the studio, and today they need that file (see Open questions).

## 5. Provider abstraction

| Approach | Verdict |
|---|---|
| **Hand-rolled, stdlib `urllib`, 2–3 adapters in the existing Python script** | **Recommended.** It matches the current script, which is 65 lines with no dependencies. The response shapes already converge: OpenAI and OpenRouter both return `data[0].b64_json`, Gemini returns inline base64, and BFL returns a polling URL |
| LiteLLM | Reject. It has 20+ base dependencies (boto3, tokenizers, huggingface-hub…) for what amounts to three HTTP calls. Its image params are OpenAI-centric, so Gemini aspect ratios and BFL tiers would need passthrough anyway. **Adoption risk:** PyPI releases 1.82.7 and 1.82.8 were compromised on 2026-03-24 with a credential stealer that read environment variables, which is exactly the asset at stake here [S×3: Datadog, NetSPI, Comet] |
| OpenAI SDK pointed at different `base_url`s | Works only for OpenAI-shaped endpoints. Gemini's OpenAI-compatibility image endpoint is stale: its docs still list only `gemini-2.5-flash-image` and a 3-pro preview that has been shut down [O] |

**DRY placement:** keep all provider knowledge in Python, `generate-scene-images.py`, with a `--check` mode for the free test call. Go only stores keys, builds the env, and runs the script. Go never learns provider URLs.

---

## 6. Recommended design

### 6a. Settings schema (`config.json`, no secrets)

```json
"image": {
  "provider": "openai",
  "model": "gpt-image-2.5-flare",
  "size": "2048x1152",
  "quality": "high",
  "parallel": 4
},
"imageProviders": {
  "openai":     {"kind": "openai-images", "baseUrl": "https://api.openai.com/v1",        "path": "/images/generations"},
  "aihub":      {"kind": "openai-images", "baseUrl": "https://aihub.tigrendev.com/v1",   "path": "/images/generations"},
  "openrouter": {"kind": "openai-images", "baseUrl": "https://openrouter.ai/api/v1",     "path": "/images"},
  "gemini":     {"kind": "gemini"},
  "local":      {"kind": "mflux", "model": "flux2-klein-4b"}
},
"keyMeta": {
  "openai": {"last4": "Ab12", "addedAt": "2026-10-24T16:30:00+07:00", "check": "ok", "checkedAt": "…"}
}
```

- Replace the flat `imageModel` field with `image`, and migrate the old value on load as `config.go` already does for defaults.
- When a song is created, `project.json` keeps a snapshot: `image_provider` + `image_model` + `image_size`, as `image_model` is stored today. Model names only make sense per provider.
- Keychain: service `que-huong-studio`, account = provider id (`openai`, `aihub`, `openrouter`, `gemini`). The local provider needs no key.

### 6b. API (all behind the existing guard)

- `PUT /api/keys/{provider}` with body `{key}`. It saves to Keychain, runs `--check`, and returns `{last4, check, message}`.
- `POST /api/keys/{provider}/check` returns `{check, message}`.
- `DELETE /api/keys/{provider}`.
- There is no GET that returns a key.

### 6c. Child env passing (image step only)

```text
QH_IMAGE_PROVIDER=openai
QH_IMAGE_KIND=openai-images
QH_IMAGE_BASE_URL=https://api.openai.com/v1
QH_IMAGE_PATH=/images/generations
QH_IMAGE_API_KEY=<from Keychain, this cmd.Env only>
```

The script resolves the key in this order: `QH_IMAGE_API_KEY`, then legacy `AIHUB_API_KEY`, then the legacy file. Model, size and quality come from `project.json`. In the studio process, only the run for the `images` action (actions.go:341) gets these variables.

### 6d. UI copy (the app UI is Vietnamese)

| Element | Text |
|---|---|
| Section title | **Nhà cung cấp ảnh** |
| Provider select hint | Ảnh cảnh được tạo qua nhà cung cấp này. Mỗi bài lưu lại lựa chọn lúc tạo. |
| Key field (empty) | placeholder: `Dán khoá API vào đây` |
| Key saved state | `Đã lưu •••• Ab12 · thêm 24/10/2026` + buttons **Kiểm tra**, **Thay khoá**, **Xoá** |
| Storage note | Khoá được cất trong Keychain của macOS, chỉ gửi cho bước tạo ảnh và không bao giờ hiện lại ở đây. |
| Honest caveat (small, muted) | Chương trình khác chạy dưới tài khoản máy của bạn vẫn có thể đọc Keychain qua lệnh `security`. Chỉ dán khoá có giới hạn chi tiêu. |
| Test OK | ✓ Khoá hợp lệ (không tốn tiền). |
| Test fail 401 | Khoá bị từ chối. Kiểm tra lại hoặc tạo khoá mới ở trang của nhà cung cấp. |
| Model not available | Khoá đúng nhưng tài khoản chưa dùng được model này (OpenAI có thể yêu cầu xác minh tổ chức). |
| Cost hint under size/quality | Ước tính ~$0.85 cho 20 ảnh ở 2048×1152, chất lượng cao. |
| Remove confirm | Xoá khoá OpenAI khỏi Keychain? Bước tạo ảnh sẽ dừng cho tới khi bạn dán khoá mới. |
| Legacy import | Tìm thấy khoá cũ ở ~/.config/suno-que-huong/aihub.key. **Nhập vào Keychain** |

The "Honest caveat" row tells the user plainly that other programs running under their macOS account can still read the Keychain through the `security` command, and that they should only paste keys that have a spending limit. This also lines up with OpenAI's project keys and per-project budgets.

---

## Trade-offs and adoption risk

| Item | Maturity | Risk |
|---|---|---|
| gpt-image-2.5 | GA, snapshot `-2026-09-08` | OpenAI retires image models quickly (gpt-image-1 shut down yesterday). **Make the model a setting, never a constant** |
| Nano Banana 2.1 | GA 2026-10-06, no shutdown date | Google moves to a new API surface (`interactions`) and renames models often (Imagen 4 shut down in Aug) |
| FLUX 3 Image | Launched 2026-10 | Very new; prices only in a JSON-LD block on bfl.ai/pricing [O] |
| OpenRouter Images API | Newer, dedicated endpoint | A middleman sees your prompts and charges 5.5%. Parameter support varies per endpoint |
| go-keyring | 1.3k stars, active, MIT | Low |
| mflux | Very active, many releases a month | CLI flags change often. Pin the version |

## Limitations of this research

- **Latency:** only OpenAI publishes qualitative figures. Seconds per image for Gemini and BFL were not measured, because doing so costs money and needs keys. Run a 3-image probe per provider before you choose a default.
- **Local speed on this M1 Pro** is extrapolated from M1 Max data, not run. Weight downloads are 8–16 GB.
- **Ideogram 4.5 prices** come from two trackers that read the official page on 2026-10-14 and 2026-10-18. The official page loads prices client-side.
- **Output and commercial terms** were not re-read in full for OpenAI, BFL or Recraft. The Ideogram FAQ (commercial use OK) and Gemini paid-tier data use were checked.
- **Cost of OpenAI reference-image input tokens** at high fidelity is unquantified.
- **The aihub gateway's real billing** for its `gpt-image-2.5` alias is unknown.

## Open questions

1. Do agents still run `generate-scene-images.py` standalone, outside the studio? If yes, the script needs a Keychain fallback (`security find-generic-password -s que-huong-studio -a <provider> -w`), or the legacy `aihub.key` file must stay.
2. Should the aihub gateway remain the default, or should OpenAI direct replace it? Model IDs differ: `gpt-image-2.5` on the gateway vs `gpt-image-2.5-flare` on OpenAI.
3. Are the videos monetised? If so, local models must stay on Apache-2.0 weights (klein 4B, schnell, Z-Image-Turbo), and FLUX 3 Klein's "Kommunity" license needs a read.
4. Is the 5.5% OpenRouter fee and the extra party seeing prompts acceptable, in exchange for one key covering FLUX, Recraft and Gemini? If not, write a native Gemini adapter and skip OpenRouter.

## Sources

Official: [OpenAI image generation guide](https://developers.openai.com/api/docs/guides/image-generation) · [OpenAI pricing](https://developers.openai.com/api/docs/pricing) · [gpt-image-2.5-flare](https://developers.openai.com/api/docs/models/gpt-image-2.5-flare) · [gpt-image-2.5-sunburst](https://developers.openai.com/api/docs/models/gpt-image-2.5-sunburst) · [OpenAI deprecations](https://developers.openai.com/api/docs/deprecations) · [Gemini pricing](https://ai.google.dev/gemini-api/docs/pricing) · [Gemini image generation](https://ai.google.dev/gemini-api/docs/image-generation) · [Gemini deprecations](https://ai.google.dev/gemini-api/docs/deprecations) · [Gemini OpenAI compatibility](https://ai.google.dev/gemini-api/docs/openai) · [BFL pricing](https://bfl.ai/pricing) · [BFL docs pricing](https://docs.bfl.ml/quick_start/pricing.md) · [FLUX 3 text-to-image](https://docs.bfl.ml/flux_3/flux3_image_generate.md) · [FLUX 3 Klein](https://docs.bfl.ml/flux_3/flux3_image_klein.md) · [BFL credits endpoint](https://docs.bfl.ml/api-reference/get-the-users-credits.md) · [BFL licensing](https://bfl.ai/licensing) · [fal FLUX.2 pro](https://fal.ai/models/fal-ai/flux-2-pro) · [Replicate FLUX.2 pro](https://replicate.com/black-forest-labs/flux-2-pro) · [Recraft pricing](https://www.recraft.ai/docs/api-reference/pricing.md) · [Recraft endpoints](https://www.recraft.ai/docs/api-reference/endpoints.md) · [Stability pricing](https://platform.stability.ai/pricing) · [Ideogram API pricing](https://ideogram.ai/features/api-pricing) · [OpenRouter image generation](https://openrouter.ai/docs/guides/overview/multimodal/image-generation) · [OpenRouter FAQ](https://openrouter.ai/docs/faq) · [mflux](https://github.com/filipstrand/mflux) · [zalando/go-keyring](https://github.com/zalando/go-keyring) · [99designs/keyring](https://github.com/99designs/keyring) · [LiteLLM image generation](https://docs.litellm.ai/docs/image_generation) · [Draw Things Z-Image post](https://releases.drawthings.ai/p/quantify-z-image-turbo-efficiency) · Hugging Face model API for licenses ([FLUX.2-klein-4B](https://huggingface.co/black-forest-labs/FLUX.2-klein-4B), [FLUX.2-klein-9B](https://huggingface.co/black-forest-labs/FLUX.2-klein-9B), [FLUX.1-schnell](https://huggingface.co/black-forest-labs/FLUX.1-schnell), [Z-Image-Turbo](https://huggingface.co/Tongyi-MAI/Z-Image-Turbo), [Qwen-Image](https://huggingface.co/Qwen/Qwen-Image)) · `man security` (macOS 26.5.1).

Secondary: [lilting.ch FLUX.2 klein on M1 Max](https://lilting.ch/en/articles/flux2-klein-4b-mflux-iris-m1-max) · [Wireflow Ideogram pricing](https://www.wireflow.ai/blog/ideogram-api-pricing) · [Developers Digest Ideogram](https://www.developersdigest.tech/models/image-generation/ideogram) · [Datadog on LiteLLM compromise](https://securitylabs.datadoghq.com/articles/litellm-compromised-pypi-teampcp-supply-chain-campaign/) · [NetSPI](https://www.netspi.com/blog/executive-blog/ai-ml-pentesting/litellm-supply-chain-compromise/) · [Comet](https://www.comet.com/site/blog/litellm-supply-chain-attack/) · [TrueFoundry OpenRouter pricing](https://www.truefoundry.com/blog/openrouter-pricing) · [skill-sprinters on FLUX 3 Klein license](https://skill-sprinters.de/blog/news/flux-3-image-klein-offene-gewichte-lizenz-2026/) · [Draw Things API (playbooks)](https://playbooks.com/skills/openclaw/skills/drawthings).
