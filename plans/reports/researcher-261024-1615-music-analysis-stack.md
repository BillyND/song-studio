# Local music-analysis stack for "song → karaoke video + lead sheet" (any genre, any language)

Date: 2026-10-24. Target machine checked: Apple M1 Pro, 32 GB (the brief said M2/M3; the numbers below hold for any M-series chip, give or take). Nothing was installed or benchmarked locally, so every speed figure is either from a cited source or marked as an estimate.

## Bottom line

Three current choices hold the stack back when it has to handle any genre and any language. Demucs `htdemucs` is about 2.6 dB worse at isolating vocals than the current RoFormer models. Whisper `tiny` with `language="vi"` hardcoded is the weakest alignment model available. And `librosa.beat_track` finds no downbeats, so bar lines are guessed.

Recommended pipeline:

1. **Separation.** Use python-audio-separator with the Mel-Band RoFormer "Kim" vocal model, whose weights are MIT. If backing vocals are a problem, run a RoFormer *karaoke* model on the vocal stem to get the lead voice alone.
2. **Alignment.** Use CTC forced alignment (`torchaudio.functional.forced_align`, as WhisperX does) inside each lyric line's time window, with one Apache-2.0 wav2vec2 model per language. Keep stable-ts as the fallback and for Vietnamese, but move it to Whisper `large-v3-turbo`.
3. **Beats.** Use `beat_this` (MIT code and weights) for beats and downbeats, and get the meter from the spacing of the downbeats.
4. **Key and chords.** Use `skey` (Deezer, MIT) for the key, and the 2019 music-x-lab large-vocabulary chord model (MIT, packaged as `lv-chordia`) or BTC (MIT) for chords. Snap the chords to the beat grid.
5. **Melody notes.** Use openvpi **GAME** (MIT, released March 2026) in its *align* mode, which fills note boundaries inside word boundaries you already know. That is exactly the "syllable → notes + slur" model the lead sheet already uses. The lower-risk step is to keep the current per-word logic and swap pYIN for RMVPE.
6. **Engraving.** Keep music21 and the MuseScore 4 CLI. Verovio (LGPL, with Go/JS/Python bindings) is the right tool only if you want an in-browser preview.

## Recommendation table

| Stage | Recommended (rank 1) | Why | License (code / weights) | Size, speed on M-series | Integration risk | Current tool |
|---|---|---|---|---|---|---|
| Vocal separation | `audio-separator` 0.47.1 + `vocals_mel_band_roformer.ckpt` (Kim) | Same-protocol Multisong vocal SDR: **10.98** vs htdemucs 4-stem **8.24** (ZFTurbo table). MPS path officially supported and CI-tested. One CLI/Python API covers all UVR model types and ensembles. | MIT / **MIT** (HF tag on KimberleyJSN/melbandroformer) | 913 MB checkpoint. Estimated 1–3 min per 4-min song on MPS, roughly 3–6× slower than htdemucs (*unverified*). | Low–medium. Requires PyTorch ≥ 2.13 on Apple Silicon, so give it its own venv. | **Replace** htdemucs |
| Lead vs backing vocals (optional) | `mel_band_roformer_karaoke_*` (aufr33/viperx, becruily) run on the vocal stem | Removes harmonies and ad-libs that confuse the aligner and the pitch tracker. The audio-separator `karaoke` preset ensembles 3 models (about SDR 10.6). | MIT / **unspecified** (community weights) | 913 MB, similar speed to the above | Medium: weight license unclear, and quality varies by song | New |
| Lyrics alignment, known text | CTC forced alignment per lyric line: `torchaudio.functional.forced_align` (kept in torchaudio 2.10+) or WhisperX `align()`, with Apache-2.0 wav2vec2 models (en, es, zh, ja, ko) | 20 ms frame emissions give tight word boundaries and handle dense rap. Character-level alignment works for zh/ja (no spaces needed). A `<star>` wildcard absorbs ad-libs. Per-word confidence lets you flag bad lines. | BSD-2 (WhisperX) or BSD (torchaudio) / **Apache-2.0** for the jonatasgrosman XLSR-53 and kresnik Korean models | About 1.26 GB per XLSR-large model. turbo-lyrics measured **13.8 s for a 4:12 song** (known lyrics) on an M5 Pro CPU. | Medium: no permissive Vietnamese CTC model exists (see below), so you need a model registry and a fallback | **Change** |
| Alignment fallback, and Vietnamese | stable-ts 2.19.1 + Whisper **large-v3-turbo** | Covers all 99 Whisper languages with one engine and keeps the existing code. Only the model and the `language` argument change. | MIT / MIT | About 1.6 GB. Estimated 30–90 s per song on CPU/MPS (*unverified*). | Low code risk. Medium maintenance risk: the repo says development is "indefinitely paused", and the last release was Aug 2025. | **Keep, upgrade the model** (drop `tiny` and the hardcoded `vi`) |
| Line timings | Suno embedded SRT | Already correct for Suno songs. For other songs, CTC alignment over the whole song with `<star>` tokens, or a stable-ts line pass, provides the line windows. | n/a | n/a | n/a | **Keep** for Suno |
| Post-snap | Existing RMS/onset snapping (librosa) | Cheap, works with any aligner | ISC | negligible | none | **Keep** |
| Beats, downbeats, meter | `beat_this` 1.1.0 (CPJKU, ISMIR 2024) | GTZAN beat F1 **89.1**, downbeat F1 **78.3**. Ahead of the previous state of the art (Hung et al.) on downbeats, and needs no DBN, madmom or NATTEN. Run it on the full mix, not on `no_vocals`. | MIT / **MIT** | 78 MB (`final0`) or 8 MB (`small0`). Estimated a few seconds per song on CPU. | Low. README only documents CUDA/CPU; MPS is untested, but CPU is fast enough. | **Replace** `librosa.beat_track` |
| Sections (intro/verse/chorus) | Take them from lyric tags (`[Verse]`/`[Chorus]` in Suno lyrics) plus alignment times. Only reach for allin1 if you need labels from audio alone. | For Suno songs the labels already exist. allin1 has had no commits since May 2024 and has NATTEN/madmom install problems on macOS; an open issue (2026-10-03) asks whether commercial use is allowed. | allin1: MIT / unclear (trained on Harmonix) | allin1 runs htdemucs internally, so it is slow | **High** for allin1 | New |
| Key | `skey` (Deezer, S-KEY, ICASSP 2025) | Self-supervised key model that matches supervised state of the art (authors' claim). Has an MPS flag. Use music21 only to spell the notes. | MIT / MIT (per repo) | Small, seconds per song (*unverified*) | Low–medium: 72 stars, the PyPI 0.2 package has no license metadata | **Replace** chroma template |
| Chords | `lv-chordia` (repackages music-x-lab ISMIR 2019 large-vocabulary model, 5-model ensemble + HMM) | Self-reported accuracy about 81% Billboard, 78% RWC-Pop, 83% Beatles (submission vocabulary). Weights are bundled, so it works offline. | MIT / MIT | Seconds to tens of seconds per song (*unverified*) | Medium: 16 stars and a repackaged 2019 model. BTC (217 stars, MIT, weights in the repo) is a fine #2. | **Replace** chroma |
| Melody → notes | **GAME** v1.0.3 (openvpi, successor of SOME), `align` mode with word boundaries from the aligner | Keeps known word boundaries fixed and adds note boundaries inside them, outputting `note_slur`, which is your melisma model. Designed for "dirty or separated voice". ONNX exports are available. | MIT / released under the MIT repo, but the weights carry no separate license | ONNX small 43 MB, medium 171 MB, large 344 MB. Speed *unverified*. | **Medium–high**: research code, no pip package, built around DiffSinger CSVs. Trained on en, ja, yue, zh only (32 h of private data). Evaluated only on its own validation split. | **Replace** pYIN, or see the lower-risk path |
| Melody, lower-risk path | Keep the per-word logic and swap `librosa.pyin` for RMVPE (or FCPE) | RMVPE is built for vocal pitch in polyphonic music and is the RVC/DiffSinger default. A drop-in f0 source. | Apache-2.0 (Dream-High/RMVPE) / MIT (RVC-hosted weights) | About 180 MB, faster than pYIN on GPU | Low | Incremental |
| Engraving | music21 10.5.1 → MusicXML → MuseScore 4.7.5 CLI (`mscore -o x.pdf`) | Already works, renders `<harmony>` chord symbols and Unicode lyrics, and you edit in the same app | BSD-3 / GPL-3 app (output is unencumbered) | about 2–5 s per render | Low. For CJK, set the lyric font in the `.mss` style (PingFang SC / Hiragino Sans / Apple SD Gothic Neo ship with macOS). | **Keep** |

## Licensing flags (non-commercial or unclear)

This only matters if videos or sheets are published commercially; a monetized YouTube channel counts. These weights are **non-commercial**:

- **MMS** (`facebook/mms-*`, `MahmoudAshraf/mms-300m-1130-forced-aligner`, torchaudio `MMS_FA`): CC-BY-NC-4.0. This is the only single model that aligns all 1100+ languages, so it would be the simplest option if NC were acceptable.
- **WhisperX's default Vietnamese aligner** (`nguyenvulebinh/wav2vec2-base-vi-vlsp2020`) and every other Vietnamese wav2vec2 checked (`dragonSwing`, `khanhld`): CC-BY-NC-4.0. WhisperX's torchaudio VoxPopuli defaults (es/fr/de/it) are also CC-BY-NC, according to turbo-lyrics. Use `jonatasgrosman/wav2vec2-large-xlsr-53-spanish` (Apache) instead.
- **madmom** model files: CC BY-NC-SA. The PyPI release (0.16.1, 2018) does not install on Python ≥ 3.10.
- **ROSVOT** checkpoint: trained only on M4Singer, which is CC BY-NC-SA.
- **Sheet Sage** weights: CC BY-NC-SA 3.0 (HookTheory-derived). It also needs Linux, Docker, and a CUDA GPU with at least 12 GB.
- **SOFA/HubertFA** singing aligner checkpoints on HF: CC-BY-NC-SA-4.0.
- **Essentia** is AGPL-3.0, and most Essentia TensorFlow models are CC BY-NC-ND. AGPL is harmless for a local tool, but avoid it if the studio is ever offered as a service.
- **Unclear**: the viperx BS-RoFormer weights (the audio-separator default), and the unwa, gabox and becruily community checkpoints, have no license tag. Prefer Kim's MIT Mel-Band model. The Demucs weights have no separate license; the code is MIT, and the training data was MUSDB plus 800 internal songs.

## Stage notes

### 1. Vocal separation

- **Same-protocol comparison** (MVSep Multisong set, ZFTurbo `pretrained_models.md`). Vocal SDR: Mel-Band RoFormer Kim 10.98, BS-RoFormer viperx 10.87, MDX23C 10.17, htdemucs 8.24, htdemucs_ft vocals 8.38. The gap of about 2.6 dB is large and audible: less hi-hat and guitar bleed in the vocal stem, which is what breaks the aligner and the pitch tracker.
- **audio-separator's own per-track scores** (`models-scores.json`, median vocal SDR) give the same ordering: Kim 12.60, BS-RoFormer 1296/1297 12.10/11.77, MDX23C-InstVoc-HQ 10.56, htdemucs 9.87. Track subsets differ between models, so use these for ranking only.
- **Paper figures on MUSDB18-HQ**: htdemucs reaches 9.00 dB average over 4 stems (README). The RoFormer papers report higher numbers, but they used different training data, so cross-paper numbers are not comparable. The Multisong table is the fair comparison.
- **SCNet** (MIT, 2024) is efficient, but its public checkpoints are 4-stem and trained only on MUSDB. It is not competitive for vocals. **MDX23C** is a good mid option (448 MB, fast), but it is behind the RoFormer models.
- **Demucs status**: `facebookresearch/demucs` is archived. `adefossez/demucs` is now the official repo and released v4.1.0 on 2026-07-11, but the author states he is "not actively working on Demucs". Keep htdemucs only as a fast fallback.
- **Genre caveat**: the Mel-RoFormer paper's authors and Kick et al. found that separated vocals *hurt* their DALI-trained English lyrics aligner. For CTC/Whisper aligners trained on clean speech, separated vocals normally help. Check this on your own songs by A/B testing mix vs stem on 5–10 tracks.

### 2. Lyrics alignment

**State of research.** Lyrics-specific aligners reach a mean absolute error of about 0.16–0.24 s on JamendoLyrics. Examples are Deezer's contrastive model (DSE, trained on 88k internal songs, not released) and Kick et al. 2024 (code public, trained on DALI). These are **English-only** models, not packaged for multilingual use. I found no published head-to-head of WhisperX, stable-ts, MMS and Qwen on *multilingual singing*. Jam-ALT is a transcription benchmark (WER), not an alignment benchmark. So the ranking below rests on architecture plus licensing, and it needs a small local evaluation.

**Ranking when the lyrics text is known:**

1. **CTC forced alignment with per-language models** (WhisperX `align()` or `torchaudio.functional.forced_align`; torchaudio kept `forced_align` after first planning to remove it, per the 2026-01-22 maintainer edit on pytorch/audio#3902). Feed it line windows from the Suno SRT, or from a first pass. Permissive models: en `facebook/wav2vec2-base-960h` or XLSR-53-english, es/zh/ja XLSR-53 (jonatasgrosman), ko `kresnik/wav2vec2-large-xlsr-korean`, all Apache-2.0. Strengths: 20 ms frames suit rap, a long vowel just extends one token's span (melisma), and the scores make a quality gate possible (the turbo-lyrics pattern). For backing vocals, align on the lead-vocal stem.
2. **stable-ts + Whisper large-v3-turbo** works for every language including Vietnamese, with permissive weights. Whisper cross-attention timestamps are coarser than CTC. Your RMS/onset snapping makes up some of the difference. `tiny` is the main reason the current Vietnamese alignment needs heavy snapping.
3. **Qwen3-ForcedAligner-0.6B** (Apache-2.0, Jan 2026) covers 11 languages, including zh, en, ja, ko and es, but **not Vietnamese**. Its model card lists "Speech" as the only supported audio type, it caps input at 5 minutes, it resolves time in 80 ms frames, and `qwen-asr` pins `transformers==4.57.6`. Treat it as experimental for singing.
4. **MMS-300m-1130** via `ctc-forced-aligner` gives one model for all languages using uroman romanization. Technically it is the simplest, but it is **CC-BY-NC**.
5. **Not recommended:** NeMo Forced Aligner (heavy toolkit, thin coverage of the target languages), SOFA/HubertFA (NC, Mandarin-centric, need phoneme dictionaries), and LyricWhiz (needs GPT-4, so not local).

**Languages without spaces, and tonal languages:**

- **zh**: align per Hanzi character (WhisperX already does this for `zh` and `ja`). One character is one sung syllable, which maps to one note.
- **ja**: align on characters, but for the lead sheet convert kanji to kana with fugashi + unidic-lite (MIT/BSD), because a single kanji can span several morae and notes. Avoid pykakasi, which is GPL-3.0.
- **ko**: the text has spaces. One Hangul block is one syllable, which maps to one note.
- **vi**: one space-separated token is one syllable, which is already the case.
- **en/es**: you need sung-syllable splitting for note-per-syllable lyrics. Pyphen (LGPL) gives typographic hyphenation, which is close enough for a draft.
- **Tones**: CTC and Whisper alignment ignore lexical tone for timing purposes, and sung melody largely overrides tone anyway. Tone only affects *transcription* accuracy, which you do not need because the text is known.

### 3. Beats, downbeats, meter, structure

- **beat_this** reports per-dataset downbeat F1 between 61 (ASAP) and 99.7 (Candombe), for example Ballroom 95.3, Harmonix 90.7 and RWC-Pop 93.7. It is weakest on classical music and rubato, which affects hymns and slow ballads with free time, so keep a manual BPM/meter override as the studio already does. It outputs beats and downbeats but no meter label. Count beats between consecutive downbeats and take the mode to get 2, 3 or 4. 6/8 vs 3/4 stays ambiguous and should stay a user choice.
- **madmom**: weights are NC, and the PyPI release is broken on modern Python. **Essentia**: AGPL, with a heavy TensorFlow model stack. **librosa**: no downbeats. **allin1**: see the table; its only real advantage is section labels.
- **SongFormer** (2025, ASLP) is a newer structure model, but it depends on MuQ/MusicFM features and has a NOASSERTION license. Not worth it while lyric tags already give you sections.

### 4. Key and chords

- **Key:** `skey` first. Essentia KeyExtractor is the alternative if AGPL is acceptable. libkeyfinder is GPL-3 C++ and last changed in Nov 2024.
- **Chords:** `lv-chordia` or BTC. ChordFormer (arXiv 2502.11840, 2025) has no official weights release that I could confirm. autochord and Chordino are older NNLS/Vamp approaches. A lead sheet wants a reduced vocabulary (maj, min, 7, maj7, m7, sus, dim) quantized to one chord per bar or half bar on the beat_this grid. Both models output frame-level labels, so this reduction is your code to write.

### 5. Vocal melody → notes

- **GAME** is the best fit because your lead sheet is word-driven: aligner word spans go in, and notes plus slurs come out. Its weaknesses: no public benchmark against ROSVOT or SOME on a shared test set, no training data for vi, ko or es (the language embedding is optional, and the README suggests picking "a similar one"), and research-grade packaging. Run it through its ONNX export to avoid pulling Lightning in.
- **ROSVOT** (MIT code, NeurIPS 2024) also does word-boundary-aware note transcription, but its released checkpoint is M4Singer-only, which is NC.
- **STARS** (MIT, 2025) unifies transcription, alignment and style, but ships only zh and zh+en checkpoints and needs phoneme sequences.
- **basic-pitch** (Apache-2.0) is polyphonic and instrument-agnostic, and fragments sung notes with vibrato and slides. **MT3/YourMT3** is multi-instrument, and YourMT3 is GPL-3. **Sheet Sage** is NC, Linux, Docker and CUDA only. None of these suits a sung lead sheet.
- **Pitch trackers** (if you keep your own segmentation): RMVPE (polyphonic-robust, the RVC/DiffSinger standard), FCPE (MIT, faster, slightly less robust), PESTO (LGPL, tiny, monophonic), CREPE (MIT, slow full model). pYIN is the least robust on stems with bleed.

### 6. Engraving

- **MuseScore 4 CLI** is the right tool to keep. It imports MusicXML `<harmony>` and `<lyric>` with `syllabic` and `extend`, renders the PDF, and is the editor anyway.
- **Verovio** 6.3.0 (LGPL, active, with Python, JS/WASM and Go bindings) renders MusicXML to SVG in milliseconds. It suits a live preview inside the Go/web studio. It does not produce PDF directly.
- **LilyPond** 2.27 has the best typography, but MusicXML import (`musicxml2ly`) is lossy and CJK needs font configuration. It is not worth a second engraving path.
- For **CJK in MusicXML**: music21 writes UTF-8 lyrics without problems. Put one syllable per `<lyric>` and use `<extend/>` for melismas. Set `lyricsOddFontFace` and `lyricsEvenFontFace` in the MuseScore style to a CJK-capable macOS font; otherwise the glyphs can fall back unevenly.

## Trade-off matrix: alignment options (the hardest choice)

| Option | Precision | Language coverage | License | Mac/MPS | Maintenance | Score |
|---|---|---|---|---|---|---|
| CTC per-language wav2vec2 (WhisperX or torchaudio) | High (20 ms) | en, es, zh, ja, ko permissive; vi NC only | Apache / BSD | CPU fine; MPS likely | Active (WhisperX pushed 2026-10-22) | **1** |
| stable-ts + large-v3-turbo | Medium | 99 languages | MIT | CPU/MPS | Paused | **2** (fallback, vi) |
| Qwen3-ForcedAligner | Unknown on singing (80 ms) | 11, no vi | Apache | Transformers backend; MPS *unverified* | New (Jan 2026), Alibaba-backed | 3 (experiment) |
| MMS-1130 (ctc-forced-aligner) | High | 1100+ | **NC** | CPU fine | Active | 4 (if NC acceptable, rises to 1) |

## Adoption risk summary

- **Low:** audio-separator (1.4k stars, release 5 days ago), beat_this (MIT, CPJKU), WhisperX (24.8k stars), music21, MuseScore, Verovio.
- **Medium:** stable-ts (paused), skey (small project), the lv-chordia and BTC chord models (2019 models), community RoFormer karaoke weights (license).
- **High:** GAME (research code, new), allin1 (unmaintained), Qwen3 aligner on singing (off-domain).

## Limitations of this research

- No local benchmarks. Speeds marked *unverified* are estimates. GAME, skey and lv-chordia accuracy figures are the authors' own.
- No multilingual singing-alignment benchmark exists publicly, so the alignment ranking is a reasoned bet. Validate it on about 10 songs per language with hand-checked word onsets, using the `JamendoLyrics` mean absolute error metric.
- Vietnamese is the language with the least permissive tooling. Every permissive Vietnamese CTC model checked is NC.
- I did not verify MPS behaviour for beat_this, skey, WhisperX alignment or Qwen3. CPU fallbacks are fast enough at song length.

## Unresolved questions

1. Will the videos or sheets be monetized or sold? The answer decides MMS (simplest universal aligner) vs the per-language Apache models plus the Whisper fallback, and it rules community separation weights in or out.
2. For non-Suno songs, will lyrics text always be supplied? If not, an ASR pass is needed first. Candidates are Whisper large-v3-turbo (MIT) and Qwen3-ASR-1.7B (Apache-2.0). Qwen3-ASR lists Vietnamese and "songs with BGM" among its inputs and claims the best open-source singing results; that claim has not been checked independently.
3. Should section labels come from audio at all, or are lyric tags enough? If tags are enough, drop allin1 and SongFormer entirely.
4. Is a separate Python environment per heavy stage acceptable (audio-separator needs torch ≥ 2.13 on Apple Silicon, qwen-asr pins transformers)? The setup script would need to manage two or three uv venvs.

## Sources

- ZFTurbo pretrained models (Multisong SDR table): https://github.com/ZFTurbo/Music-Source-Separation-Training/blob/main/docs/pretrained_models.md
- python-audio-separator README, `models-scores.json`, `ensemble_presets.json`: https://github.com/nomadkaraoke/python-audio-separator
- Demucs official repo and v4.1.0 notes: https://github.com/adefossez/demucs (archived original: https://github.com/facebookresearch/demucs)
- Mel-Band RoFormer paper: https://arxiv.org/abs/2310.01809 ; Kim weights (MIT): https://huggingface.co/KimberleyJSN/melbandroformer
- Kick et al., Contrastive Lyrics Alignment with a Timestamp-Informed Loss (JamendoLyrics/++ AAE table): https://tik-db.ee.ethz.ch/file/45c7793c1a3a476d56f784f74d668d74/ ; Durand et al. 2023: https://arxiv.org/abs/2306.07744
- Jam-ALT benchmark: https://arxiv.org/abs/2311.13987
- WhisperX `alignment.py` model registry: https://github.com/m-bain/whisperX/blob/main/whisperx/alignment.py
- stable-ts (development paused notice): https://github.com/jianfch/stable-ts
- torchaudio maintenance plan, with `forced_align` preserved: https://github.com/pytorch/audio/issues/3902
- ctc-forced-aligner: https://github.com/MahmoudAshraf97/ctc-forced-aligner ; MMS aligner license: https://huggingface.co/MahmoudAshraf/mms-300m-1130-forced-aligner
- turbo-lyrics (permissive-only CTC registry, M5 Pro timings): https://github.com/LoggeL/turbo-lyrics
- Qwen3-ForcedAligner model card: https://huggingface.co/Qwen/Qwen3-ForcedAligner-0.6B ; report: https://arxiv.org/abs/2601.21337
- Omnilingual ASR (Apache CTC models, 40 s input limit): https://github.com/facebookresearch/omnilingual-asr
- SOFA/HubertFA checkpoints (NC): https://huggingface.co/Silasimo/SOFA-combined ; STARS: https://github.com/gwx314/STARS
- beat_this paper and repo: https://arxiv.org/abs/2407.21658 , https://github.com/CPJKU/beat_this
- All-In-One: https://github.com/mir-aidj/all-in-one ; SongFormer: https://github.com/ASLP-lab/SongFormer
- skey: https://github.com/deezer/skey ; lv-chordia: https://github.com/openmirlab/lv-chordia ; BTC: https://github.com/jayg996/BTC-ISMIR19 ; ChordFormer: https://arxiv.org/abs/2502.11840
- GAME: https://github.com/openvpi/GAME (ALGORITHMS.md, releases) ; SOME: https://github.com/openvpi/SOME ; ROSVOT: https://github.com/RickyL-2000/ROSVOT ; RMVPE: https://github.com/Dream-High/RMVPE ; basic-pitch: https://github.com/spotify/basic-pitch ; Sheet Sage: https://github.com/chrisdonahue/sheetsage
- Verovio: https://github.com/rism-digital/verovio ; MuseScore releases: https://github.com/musescore/MuseScore/releases ; LilyPond releases: https://gitlab.com/lilypond/lilypond
- License and version metadata were pulled from the GitHub API, the PyPI JSON API and HF model tags on 2026-10-24.
