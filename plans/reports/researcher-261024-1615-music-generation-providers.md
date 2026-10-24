# Music generation providers for Quê Hương Studio (as of 2026-10-24)

## Bottom line

Keep Suno as the default and keep it browser-assisted. Suno has no public API: it opened a curated-partner intake form on 2026-07-01, and unofficial wrappers break its ToS and have been breaking in practice. Add **ElevenLabs Music API (`music_v2_5`)** as the first official API provider, because its composition plan maps one-to-one onto the studio's section-tagged lyrics. Add **ACE-Step 1.5** as the local, offline provider. It runs on Apple Silicon through MLX, is MIT-licensed, takes BPM, key and meter as explicit parameters, lists Vietnamese among its languages, and returns LRC timestamps. Treat Lyria 3.5 and Mureka as optional fourth and fifth choices. Skip MiniMax (its paid API has been closed to new users since 2026-08-20), Udio (no downloads, no API), Stable Audio (no vocals), and Producer/Riffusion (no official API).

The design change that matters most is a **provider-neutral `spec.json`** written by the style agent, with deterministic Go renderers turning it into each provider's format. The Suno renderer keeps writing `suno/style.txt` and `suno/lyrics.txt`, so the current CUA fill keeps working unchanged. Each take is stored as an immutable folder holding the exact request, the raw response, the audio, and a manifest. Only the local ACE-Step output can be re-generated exactly, so the stored outputs are the reproducibility record, not the recipes.

The biggest open risk is **Vietnamese vocal quality outside Suno, which nobody has verified**. ElevenLabs, Lyria and Mureka do not list Vietnamese, and ACE-Step lists it but has no published quality evidence. Run a one-song bake-off before committing (see the questions at the end).

Labels used below: **[V]** verified from an official or primary source fetched today; **[S]** secondary source (press or reputable blog); **[U]** unverified, inference, or conflicting sources.

---

## 1. Suno

| Topic | Finding | Label |
|---|---|---|
| Official API | None. On 2026-07-01 CPO Jack Brody posted an intake form for a developer API, "starting with a curated group of partners". No timeline, docs or pricing. | [S] MBW 2026-07-02 |
| Unofficial APIs | The most-starred wrapper (gcui-art/suno-api, 3.3k★) was last pushed 2026-03. A 2026-07 PR says the hCaptcha flow changed and now needs a visible browser for the account owner. Issue #289 (2026-08-29) reports that "all audio download paths return forbidden / encrypted content". | [V] GitHub |
| ToS vs. unofficial APIs | The ToS effective 2026-09-03 bars "data mining, robots, scraping", obtaining content "through any means not intentionally made available", and getting outputs other than through Suno's download channel. Suno may watermark or fingerprint outputs with the tier and download status. | [V] suno.com/terms |
| ToS vs. the studio's CUA fill | Gray area. The agent only fills the form, and the user presses Create and downloads, so no output is obtained by automation. Clause 12 ("robots") could still be read broadly. Keep the plain "copy blocks" manual path as the zero-risk fallback. This is my reading, not legal advice. | [U] |
| Models | v6 (flagship, paid), v6-wild (paid), and v6-mini (all users) shipped 2026-09-09, with section and lyric-line editing by plain language, mashup, and sample/isolate. Coverage says older models are retired as v6 rolls out, so prompts tuned on v4.5 or v5.5 may behave differently. | [V] release notes; retirement [S] |
| v5.5 (2026-03-26) | Voices (your own voice only, by ToS) replaced the Personas button. Custom Models train on 6 or more of your tracks. A Duration Slider arrived 2026-07-20. | [V] |
| Covers / upload / stems | Covers since 2024. Uploads up to 8 min (2025). Stems: Auto Split 12 categories; Advanced Split (~100 instruments, Premier) since 2026-06-11. Stems do not count as extra downloads. | [V] |
| Download formats | MP3 on all plans. WAV on Pro and Premier (web only). Video on all plans. Stems when they exist. MIDI for Studio or Premier. The M4A with embedded timed lyrics is what the studio uses today. That embedding is not documented by Suno. | [V] help center; SRT-in-M4A [U] |
| Download caps (from 2026-09-03) | Free: 7 downloads lifetime, personal use only. Pro ($8–10/mo): 20 per month. Premier ($24–30/mo): 60 per month. Extras can be bought and do not roll over. | [S] MBW 2026-08-11, Variety |
| Commercial rights | Pro and Premier: Suno assigns its rights in the output to you. Commercial use is allowed **only for outputs you downloaded within your tier's allotment**. Rights are perpetual for those downloads, even after cancelling. Free/Basic: personal, non-commercial use only. Remixes cannot be used commercially. | [V] suno.com/terms |
| Artist names | Moderation blocks "names of well-known artists or people" and trademarked terms, and does not say which word triggered the block. | [S] quoted moderation text |

What this means for the studio: each Suno take now costs a scarce download (20 a month on Pro). The studio should record which take was downloaded, avoid prompting the user to download every candidate, and use local alignment (`karaoke-align.py`) rather than depending on Suno's SRT.

## 2. Official-API alternatives (full songs, vocals, your own lyrics)

| Provider / model | Lyrics input | Languages (documented) | Max length | Output | Timed lyrics | Seed | Price | Commercial rights | Status / risk |
|---|---|---|---|---|---|---|---|---|---|
| **ElevenLabs Music** `music_v2_5` | `composition_plan`: up to 30 chunks, each with `text` (`[Section]` + lines + `{inline cue}`), `duration_ms` 3–120 s, positive/negative styles (max 50 each), and `context_adherence` [V] | EN, ES, DE, JA "and more". **Styles must be English; lyrics can be any language** [V] | 10 min via API (`music_length_ms` up to 600 000) [V]; pricing page says "5 minute duration limit" [V], so the two official pages conflict | MP3 44.1/48 kHz, WAV [V] | `with_timestamps=true` returns word timestamps; schema not shown in docs [V/U] | Yes, but "exact reproducibility isn't guaranteed" [V] | **$0.15/min** API [V] | Free: no downloads. Starter: commercial but no streaming distribution. **Creator and up: streaming distribution allowed**. Film/TV/radio/Studio Games need Enterprise. No music-library use [V] | GA, paid plans only. Artist names or copyrighted lyrics return `bad_prompt` with a `prompt_suggestion` [V]. Inpainting, finetunes, ~30 s audio reference [V]. Low risk |
| **Google Lyria 3.5** (Gemini API, `lyria-3.5`) | Lyrics inside the prompt under a `Lyrics:` header with `[Verse]` tags; `[0:00-0:10]` timestamp directions [V] | Lyrics follow the prompt language. 8 vocal languages listed for Lyria 3 (EN, DE, ES, FR, HI, JA, KO, PT); **no Vietnamese listed** [S] | "A couple of minutes", set by prompt [V] | MP3 default, WAV option, 44.1 kHz stereo [V] | Returns lyrics plus a JSON structure as text; per-word timing not documented [U] | No; "results may vary" [V] | **$0.08/song**; Clip $0.04 per 30 s; no free tier [V] | Google claims no ownership of output [V]. SynthID watermark always applied [V] | Single-turn, no editing. Artist voices or copyrighted lyrics are blocked [V]. Low-to-medium risk (fast model churn: 3 → 3.5 in months) |
| **Mureka** API (V9 / O2 / V9.5) | `lyrics` up to 5000 chars, `prompt` up to 1024; `reference_id`, `vocal_id` (clone), `melody_id`; `n` up to 3 songs [V] | ZH, EN, JA, KO, PT, ES, DE, FR, IT, RU; **no Vietnamese** [V] | 5 min 30 s [V] | MP3 plus `wav_url`; stems cost extra [V] | Structured lyrics returned; timing format unverified [U] | Not documented [U] | **$0.045/song** (V8/V9/O2), $0.15 (V9.5); prepaid from $10, balance valid 12 months, no refunds [V] | Paid API output has "full usage rights and commercial authorization" [V] | Active changelog (V9.5, 2026-08). Medium risk (prepaid lock-in) |
| **MiniMax Music** 3.0 / 2.6 | `lyrics` + `prompt`; cover via reference audio [V] | Not documented | Up to 5 min | MP3/WAV | No | n/a | $0.15/song | n/a | **Paid API closed to new users since 2026-08-20**; free APIs discontinued [V]. Not viable (but see the open weights in §3) |
| **Udio** | n/a | n/a | n/a | **Downloads disabled since Oct 2025**; walled garden pending the UMG platform [S] | n/a | n/a | n/a | n/a | No API. Not viable |
| **Stable Audio 2.5 / 3.0** | No lyrics | n/a | 3.0: up to 6 min [V] | n/a | n/a | n/a | API/Enterprise | Licensed training data, Enterprise indemnity [V] | **No vocals** [S, multiple reviews]. Instrumental beds only |
| **Producer.ai / Riffusion** | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | No official public API found; only resellers. 2026 reports of a Lyria 3 integration [U]. Not viable |
| **Reseller "Suno APIs"** (sunoapi.org, aimusicapi, AceData, etc.) | Suno-like | — | — | — | — | — | — | Inherits Suno ToS problems | Proxies on top of unofficial access. Reject (ToS and account risk, single point of failure) |

## 3. Open-source local models with vocals

| Model | Release | License (weights) | Languages | Hardware / Mac | Strengths | Verdict |
|---|---|---|---|---|---|---|
| **ACE-Step 1.5** (2B / XL 4B DiT + 0.6–4B LM) | 2026-01 / XL 2026-04 | **MIT** (code and weights) [V] | "50+", explicit list includes **Vietnamese** [V]; quality per language unproven [U] | macOS launcher uses **MLX on Apple Silicon** [V]; ≤6 GB DiT-only up to ≥24 GB XL+4B LM [V]; Mac speed unpublished [U] | REST server (`/release_task`, `/query_result`) with `bpm`, `key_scale`, `time_signature`, `vocal_language`, `duration` (10 s–10 min), `seed`, wav/flac output; cover, repaint, LRC timestamps, LoRA training (8 songs, ~1 h on a 3090) [V]. The paper's blind test places it **between Suno v4.5 and v5** [S] | **Ship as the local provider** |
| **YuE2** (3B) | 2026-09 | **CC BY-NC 4.0 plus a creator permission**: individuals may monetise outputs; companies need a licence [V] | Card tags zh/en; README says "all languages" [V/U] | Official: Linux, NVIDIA 24 GB [V]; a community GGUF/MLX UI exists (unofficial) [S] | Editable symbolic score (ABC), covers via SheetSage2, claims parity with Suno v5/v6 on WildSongBench [V, self-reported] | Watch; revisit when Mac support is official |
| **MiniMax Music 3** (8B + 0.6B LM, flow matching) | 2026-08 | Community licence: commercial use OK, must display "MiniMax-Music3" in commercial products, and get approval above $20M revenue [V] | Not listed [U] | **"Inference requires CUDA"**; 8 GB with offload [V] | Up to 5 min, 32 kHz WAV, seed, structured caption (Global Metadata / Vocal / Arrangement) [V] | Not feasible on a Mac; usable only on a remote CUDA box |
| **SongGeneration v2 / LeVo 2** (Tencent) | 2026-02/03 | **Unresolved**: NOASSERTION on mirrors; one GGUF repack says research-only [S] | zh, en, es, ja [S] | 22–28 GB CUDA (large) [S]; community MLX port of v2-medium exists [S] | — | Skip until licence is clear |
| **DiffRhythm 2** | 2025-10 | Apache-2.0 [V] | zh/en focus [S] | CUDA | Fast | Skip (language coverage) |
| **HeartMuLa-oss-3B** | 2026-01 | Apache-2.0 [V] | zh, en, ja, ko, es [V] | CUDA [U] | — | Skip (no Vietnamese) |
| **SongBloom** | 2025 | Original HF repo now inaccessible; mirrors only [V] | zh/en [U] | CUDA | — | Skip |

## 4. Prompt engineering across genres

Reading across the official guides (Lyria prompt guide, ElevenLabs composition plans, MiniMax Music 3 structured caption, ACE-Step musicians' guide), they agree on one structure: **global metadata → vocal profile → arrangement → per-section direction**, with the lyrics kept separate from the instructions. Suno has no official style guide. Its practice comes from community sources, plus the studio's own skill, which already learned that Suno misreads negations.

| Measured feature (from the reference analysis) | Suno (Styles ≤1000 chars + section tags in Lyrics) | ElevenLabs (composition plan) | ACE-Step (API params + caption) | Lyria 3.5 (single prompt) |
|---|---|---|---|---|
| BPM | "92 BPM, unhurried" (number plus pace word) [S] | in the first chunk's `positive_styles` [U] | `bpm` param [V] | "around 92 BPM" (recommended) [V] |
| Key / mode | text; adherence weak [U] | style text [U] | `key_scale` "A minor" [V] | "in D minor", "C pentatonic" [V] |
| Meter | "3/4 waltz lilt", "6/8 sway" [U] | style text [U] | `time_signature` 2/3/4/6 [V] | text [U] |
| Instrumentation | 3–6 instruments with playing character; regional instruments named, then described acoustically, e.g. "đàn bầu (monochord with gliding pitch bends)" [U, heuristic] | same, ≥6–7 styles in early chunks [V] | caption [V] | instrument keyword list plus texture descriptions [V] |
| Vocal type | gender + range + timbre + delivery; also Suno's Vocal Gender control | `positive_styles` | caption | "singer profiles" e.g. "Female Alto: warm, husky" [V] |
| Energy curve | section tags with short cues `[Chorus: full band, soaring]` | per-chunk styles + `duration_ms` [V] | structure tags in lyrics [V] | `[0:00 - 0:10]` timestamp directions or arrows [V] |
| Exclusions | positive phrasing plus the Exclude Styles field (2024+) [V] | `negative_styles` (use liberally) [V] | — | describe what is present |
| Artist / song names | **Never.** Convert to era + region + genre + production + vocal descriptors. All three cloud providers block names [V/S] | `bad_prompt` returns a `prompt_suggestion`; can auto-retry [V] | not blocked, but keep the same rule (ToS/IP) | blocked by safety filter [V] |
| Language | styles English, lyrics native | **styles must be English** [V] | `vocal_language` [V] | prompt in the target language for lyric language [V] |

Rules for the style agent:

1. Emit the neutral spec with numbers kept as numbers (bpm, key, meter, duration), so ACE-Step gets them as hard parameters and text renderers phrase them per provider.
2. Run an artist/title lint before any render, using a denylist built from the reference file's tags plus the reference analysis.
3. Keep the existing rule of learning style only, never melody or lyrics, from the reference song.
4. Treat measured values as targets. Every provider documents them as guidance rather than guarantees (MiniMax Music 3 card [V]; Lyria "be specific" [V]).

## 5. Recommendation

### Ranked providers

| Rank | Provider | Mode | Why | Main trade-off |
|---|---|---|---|---|
| 1 | **Suno v6 / v5.5** | assisted (CUA fill or copy blocks) | Proven on Vietnamese folk in this studio; best quality track record; Voices, covers and stems | No API; 20 or 60 downloads a month; ToS gray area for automation; model retirement changes prompt behaviour |
| 2 | **ElevenLabs `music_v2_5`** | API (BYO key) | Official, GA. Composition plan mirrors section tags. Word timestamps, seed, inpainting. Clear rights table | $0.15/min (~$0.45 per 3-min take); Vietnamese not documented; streaming distribution needs Creator or above |
| 3 | **ACE-Step 1.5** | local server (MLX) | Free, offline, MIT, explicit BPM/key/meter, lists Vietnamese, LRC, LoRA on own songs. Fits "local-first" best | Quality below Suno v5 (own paper); Mac generation speed unknown; Python/uv dependency to manage |
| 4 (optional) | Lyria 3.5 | API | Cheapest official ($0.08/song), watermark-transparent | No seed, no length parameter, no Vietnamese listed, single-turn |
| 5 (optional) | Mureka | API | $0.045/song, 5.5 min, stems, vocal clone | Prepaid top-up, no Vietnamese listed |

### Provider abstraction (Go, fits the existing `actions.go` / `projects.go` layout)

```go
type Mode string // "assisted" | "api" | "local"

type Caps struct {
    Mode            Mode
    Languages       []string // documented; "" = unknown -> warn, don't block
    MaxDurationSec  int
    StyleMaxChars   int      // Suno 1000
    Seed, WordTimes, Stems bool
}

type Provider interface {
    ID() string
    Caps() Caps
    Render(spec SongSpec) (Request, error)          // deterministic, unit-testable; lints names/limits
    Submit(ctx context.Context, r Request) (Job, error) // assisted: ErrManual (CUA/copy path)
    Poll(ctx context.Context, j Job) ([]Take, error)    // downloads audio + raw metadata into the take dir
}
```

- **The LLM writes one neutral `gen/spec.json` and Go renders per provider.** This keeps renders reproducible and testable, and lets the user switch providers without re-running the agent. The Suno renderer writes the existing `suno/style.txt` and `suno/lyrics.txt`, so the CUA skill is untouched.
- **API keys** live in `~/.config/que-huong-studio/` with mode 600, following the existing `aihub.key` pattern. They are never written into take records.
- **Assisted mode** stays the Suno path. The take is created when the user imports the downloaded file. Record the Suno song URL or ID if the user pastes it.
- **ACE-Step** is managed as a local process with a fixed port, reused if already running and stopped on exit, per the process rules.
- **Alignment stays local.** `karaoke-align.py` remains the source of truth for `work/timings.json`. Provider timestamps (ElevenLabs words, ACE-Step LRC, Suno SRT) are stored as optional anchors only, which the existing `SunoStart` drift field already models.

### What to store per song (`<song>/gen/`)

| File | Contents |
|---|---|
| `spec.json` | neutral spec: language, sections (tag, lines, direction), bpm, key, meter, duration target, instruments, vocal profile, mood, energy curve, negatives, reference-analysis hash, spec schema version |
| `takes/<ts>-<provider>/request.json` | exact rendered payload or form contents (no secrets) |
| `takes/<ts>-<provider>/response.json` | raw provider metadata (job ID, ElevenLabs `song_metadata` / composition plan, Lyria text blocks, ACE-Step `metas` / `seed_value`) |
| `takes/<ts>-<provider>/take.json` | provider, mode, model ID **and version** (e.g. `suno v6`, `music_v2_5`, `acestep-v15-xl-sft` + LM + repo commit), seed (or `null` when the provider has none), spec hash, request hash, created_at, duration, cost estimate, **plan/tier at generation time** (rights depend on it for both Suno and ElevenLabs), Suno download-counted flag, watermark/provenance (SynthID, C2PA, Suno tier metadata), file list with sha256, timestamp source |
| `takes/.../audio.*`, `timestamps.*` | outputs as delivered; never transcoded in place |
| `project.json` `audio` | points at the selected take (keeps the current step logic) |

Reproducibility, stated plainly: only local ACE-Step with fixed weights, commit and seed can be regenerated exactly. ElevenLabs seeds are best-effort. Suno and Lyria are not deterministic, and Suno retires models. **The stored audio plus request is the record**, and the recipe alone is not.

### Making it genre- and language-agnostic

- `language` lives in the spec and drives three things. Lyric-review rules: the lục bát check applies only when the language is `vi`. Provider capability warnings (warn when the language is not in a provider's documented list, but do not block). Alignment language.
- Move Vietnamese-folk defaults from the skill's prompts into a **style profile** (data), so other genres are just other profiles.

## Trade-off matrix (1 = poor, 5 = best)

| | Quality (vi) | Automation | Cost / song | Reproducibility | Rights clarity | Local-first fit | Adoption risk (5 = low) |
|---|---|---|---|---|---|---|---|
| Suno assisted | 5 (in-house evidence) | 2 | 3 (cap-bound) | 1 | 4 | 3 | 3 |
| ElevenLabs API | ? [U] | 5 | 3 | 3 | 5 | 4 (BYO key) | 4 |
| ACE-Step local | ? [U] | 5 | 5 | 5 | 5 | 5 | 4 (active, MIT, 14k★) |
| Lyria 3.5 API | ? [U] | 5 | 5 | 1 | 4 | 4 | 3 (preview churn) |
| Mureka API | ? [U] | 5 | 5 | 2 | 4 | 4 | 3 |

## Limitations

- No audio was generated or listened to. Every quality claim comes from vendor or paper benchmarks, and Vietnamese quality outside Suno is unknown.
- Did not verify ElevenLabs' word-timestamp schema, Lyria's lyric-timing format, Mureka's lyric-timing format, or how Suno embeds SRT in M4A.
- Suno v6 prompt behaviour has no published guides yet. The Suno prompt advice above is community-sourced and mostly from v5.5.
- Prices and caps change often (Suno changed in Aug 2026, MiniMax closed in Aug 2026). Re-check before budgeting.
- ToS readings are not legal advice.
- Apple Silicon speed for ACE-Step XL and 2B was not measured.

## Sources

Suno: [release notes](https://suno.com/release-notes) · [v6](https://suno.com/release-notes/introducing-v6) · [Terms (eff. 2026-09-03)](https://suno.com/terms) · [download formats](https://help.suno.com/en/articles/13926081) · [MBW: API exploration](https://www.musicbusinessworldwide.com/suno-explores-developer-api-seeking-apps-that-unlock-experiences-generative-music-makes-possible-for-the-first-time/) · [MBW: download caps](https://www.musicbusinessworldwide.com/suno-limits-subscribers-downloads-per-month/) · [Variety: caps](https://variety.com/2026/music/news/suno-unveils-download-caps-for-free-paid-tiers-generator-1236831589/) · [gcui-art/suno-api issues](https://github.com/gcui-art/suno-api/issues) · [moderation troubleshooting (secondary)](https://jackrighteous.com/en-us/blogs/guides-using-suno-ai-music-creation/suno-prompt-contained-inappropriate-material)
ElevenLabs: [Music overview](https://elevenlabs.io/docs/overview/capabilities/music) · [compose detailed API](https://elevenlabs.io/docs/api-reference/music/compose-detailed) · [Music cookbook](https://elevenlabs.io/docs/eleven-api/guides/cookbooks/music) · [composition plans](https://elevenlabs.io/docs/eleven-api/guides/how-to/music/composition-plans) · [API pricing](https://elevenlabs.io/pricing/api) · [Music Terms](https://elevenlabs.io/music-terms) · [help: what is Eleven Music](https://elevenlabs.io/docs/help-center/product/content-production/music/what-is-eleven-music)
Google: [Lyria music generation](https://ai.google.dev/gemini-api/docs/music-generation) · [Lyria prompt guide](https://ai.google.dev/gemini-api/docs/lyria-prompt-guide) · [Gemini pricing](https://ai.google.dev/gemini-api/docs/pricing) · [Gemini API terms](https://ai.google.dev/gemini-api/terms) · [Cloud Lyria 3 prompting guide (languages)](https://cloud.google.com/blog/products/ai-machine-learning/ultimate-prompting-guide-for-lyria-3-pro)
Mureka: [docs](https://platform.mureka.ai/docs/) · [pricing](https://platform.mureka.ai/pricing) · [FAQ](https://platform.mureka.ai/docs/en/faq.html) · [changelog](https://platform.mureka.ai/docs/en/changelog.html)
MiniMax: [music guide (API closure notice)](https://platform.minimax.io/docs/guides/music-generation) · [pay-as-you-go pricing](https://platform.minimax.io/docs/guides/pricing-paygo) · [MiniMax-Music3 on HF](https://huggingface.co/MiniMaxAI/MiniMax-Music3)
Udio / Stability: [MBW: UMG–Udio](https://www.musicbusinessworldwide.com/universal-music-settles-udio-lawsuit-strikes-deal-for-licensed-ai-music-platform/) · [Stable Audio 3.0](https://stability.ai/stable-audio) · [Dubspot review (no vocals)](https://blog.dubspot.com/stable-audio-3-review) · [The Decoder](https://the-decoder.com/stability-ai-launches-stable-audio-3-0-with-up-to-six-minute-tracks-and-open-weights/)
Open source: [ACE-Step 1.5](https://github.com/ace-step/ACE-Step-1.5) · [ACE-Step 1.5 paper](https://arxiv.org/abs/2602.00744) · [YuE2](https://github.com/multimodal-art-projection/YuE) · [YuE2 paper](https://arxiv.org/abs/2609.33757) · [DiffRhythm 2](https://huggingface.co/ASLP-lab/DiffRhythm2) · [HeartMuLa](https://huggingface.co/HeartMuLa/HeartMuLa-oss-3B) · [vllm-omni SongGeneration v2 issue](https://goodfirstissue.org/vllm-project/vllm-omni/issues/3390) · [Producer API reseller changelog](https://musicapi.featurebase.app/changelog/rocket-new-producer-api-fuzz-20-now-available)

## Unresolved questions

1. **Vietnamese bake-off:** run one existing song (same spec) through Suno v6, ElevenLabs `music_v2_5`, ACE-Step 1.5 XL-sft (MLX) and Lyria 3.5. Listen for tone and diacritic accuracy and for the folk instruments. This decides whether ranks 2–4 are worth building at all.
2. Which Suno plan does the user have? It sets the monthly download budget (20 or 60) and whether commercial rights apply.
3. Does `karaoke-align.py` support languages other than Vietnamese? That decides how much "language-agnostic" depends on alignment work.
4. ElevenLabs: confirm the real maximum length (5 min vs. 10 min) and the `with_timestamps` response format on a live key.
5. How fast does ACE-Step 1.5 run on the user's Mac (chip and RAM)? This decides between 2B-turbo and XL as the default.
6. Should the CUA fill keep running given Suno ToS clause 12, or should the default switch to copy-blocks with CUA as an opt-in?
