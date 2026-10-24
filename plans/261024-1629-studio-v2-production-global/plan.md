# Studio v2: ổn định, mọi thể loại nhạc, cài đặt khoá API

Trạng thái: **giai đoạn 1–3 xong (2026-10-24)**; 5–7 chưa làm, 4 bỏ. Nguồn: 4 báo cáo nghiên cứu trong `plans/reports/researcher-261024-1615-*.md` và rà soát code ngày 2026-10-24.

## Quyết định của người dùng (2026-10-24)

- Người dùng: chủ máy và vài người quen → không ký/notarize; .app không ký kèm hướng dẫn "Vẫn mở".
- Không kiếm tiền, chỉ dùng riêng → được dùng model phi thương mại khi chúng tốt hơn (Demucs giữ được).
- Soát lời, phối khí, lên cảnh: **giữ agent CLI** (claude/codex/omp) → bỏ giai đoạn 4.
- Ảnh: **giữ hub hiện tại** làm mặc định; vẫn có ô khoá cho OpenAI/Gemini để chọn khi cần.

## Mục tiêu

Một người làm nhạc (không phải lập trình viên) dùng studio trên Mac để đi từ lời bài hát tới bài hát, video karaoke và bản nhạc, với **mọi thể loại** (pop, ballad, bolero, rock, rap, EDM, thiếu nhi, thánh ca, quê hương…), ổn định qua khởi động lại, lỗi mạng và việc bị treo.

## Ràng buộc

- Chạy local trên Mac, chỉ nghe `127.0.0.1`; người dùng tự mang khoá API.
- Giao diện chỉ tiếng Việt.
- Lời bài hát mặc định tiếng Việt; ngôn ngữ hát là thuộc tính của từng bài (để canh lời không còn cứng "vi").
- Giữ nguyên bố cục thư mục bài hát hiện có; bài cũ vẫn mở và chạy được.

## Không làm (lần này)

- Bản dịch giao diện sang ngôn ngữ khác.
- Gọi Suno qua API không chính thức (vi phạm điều khoản Suno từ 2026-09-03).
- Tạo video chuyển động bằng AI (renderer đã có zoom/pan/crossfade; video AI đắt và chỉ 720p).
- SQLite, Sparkle auto-update (lịch sử chạy dạng file vẫn đủ cho một người dùng).

## Tiêu chí hoàn thành

1. Cài đặt có mục khoá API cho các nguồn ảnh (hub, OpenAI, Gemini), lưu trong Keychain, có nút "Kiểm tra khoá"; khoá không bao giờ hiện trong log, tham số lệnh hay môi trường của agent.
2. Tắt/mở lại studio giữa chuỗi: việc đang chờ vẫn chạy tiếp; việc treo quá giới hạn thời gian tự dừng và báo lỗi rõ.
3. Trang "Kiểm tra máy" báo đủ/thiếu: ffmpeg, uv/Python, MuseScore, model, dung lượng trống, khoá API.
4. Bộ thử 6 bài khác thể loại (ballad, pop nhanh, rap, bolero, thiếu nhi, quê hương) chạy hết 9 bước không lỗi; chỉ số canh lời và bản nhạc được đo và không tệ hơn mức hiện tại trên bài Ninh Bình.
5. Không còn chữ "quê hương / lục bát / Ninh Bình" cứng trong luồng chung; phần đặc thù nhạc Việt truyền thống thành tuỳ chọn.

## Các giai đoạn

| # | Giai đoạn | Ưu tiên | Phụ thuộc |
|---|---|---|---|
| 1 | ✅ Cài đặt & khoá API (Keychain, nguồn ảnh hub/OpenAI/Gemini, ảnh 16:9, dựng khung nét hơn) | Phải | — |
| 2 | ✅ Độ bền chuỗi việc (giữ hàng chờ khi khởi động lại, timeout, thử lại đúng lỗi, ghi file nguyên tử, khoá phiên bản Python, Kiểm tra máy, gói chẩn đoán) | Phải | — |
| 3 | ✅ Mọi thể loại nhạc (bỏ ô kiểu phối đã xong; thêm thể loại/ngôn ngữ cho bài, mẫu lời dặn theo thể loại, giao diện video theo thể loại, font đủ dấu, lục bát thành tuỳ chọn) | Phải | 1 |
| 4 | ~~Gọi LLM trực tiếp thay agent CLI~~ — bỏ theo quyết định giữ agent CLI | — | — |
| 5 | Nâng chất lượng phân tích bằng vòng lặp đo được trên bộ thử: tách giọng RoFormer, beat_this cho phách mạnh/ô nhịp, Whisper large-v3-turbo, RMVPE cho cao độ | Nên | 2, 3 |
| 6 | Nhiều nguồn tạo nhạc: `gen/spec.json` trung lập; Suno (dán/điền form) mặc định, ElevenLabs Music API, ACE-Step 1.5 chạy local — chỉ sau khi nghe thử tiếng Việt | Có thể | 4 |
| 7 | Đóng gói cho vài người quen: .app thật (binary Go là file chạy chính, skill đi kèm), trình cài đặt lần đầu, hướng dẫn "Vẫn mở"; không ký | Có thể | 2 |

### 1. Cài đặt & khoá API

- `zalando/go-keyring` (đưa khoá qua stdin của `security`, không lộ trong `ps`). Cấu hình chỉ lưu tên nhà cung cấp, model, base URL; khoá nằm trong Keychain dưới service `que-huong-studio`.
- Ảnh: mặc định hub hiện tại (OpenAI-compatible); thêm OpenAI trực tiếp và Google Gemini (Nano Banana) để chọn. Model là chuỗi trong cài đặt (model OpenAI đổi tên/ngừng nhanh). Kích thước 16:9 thật, tối thiểu 2048×1152; bỏ phóng to 1,5 lần ở renderer.
- Khoá chỉ đặt vào `cmd.Env` của đúng bước cần (tạo ảnh); `childEnv()` của agent không bao giờ chứa khoá. Script vẫn đọc `AIHUB_API_KEY`/file cũ để chạy ngoài studio.
- Nút "Kiểm tra khoá" gọi endpoint miễn phí (liệt kê model) của từng nhà cung cấp; giao diện chỉ hiện 4 ký tự cuối.
- Chuyển `aihub.key` hiện có vào Keychain một lần, có xác nhận.

### 2. Độ bền

- Việc `queued` khi khởi động lại: xếp hàng lại (giữ phụ thuộc chuỗi), chỉ việc `running` mới thành "Bị gián đoạn".
- Timeout theo loại việc (ví dụ tách giọng 20 phút, xuất video 40 phút, ảnh 10 phút); dừng cả nhóm tiến trình.
- Script tạo ảnh: thử lại có backoff chỉ với lỗi mạng/429/5xx; dừng ngay với lỗi nội dung/400/401 và nói lý do bằng tiếng Việt.
- Ghi đầu ra vào file tạm rồi đổi tên (video, PDF, timings, ảnh) để việc bị dừng không để lại file dở trông như xong.
- `uv lock` có hash cho môi trường Python; `setup-video-env.sh` cài theo lockfile.
- Xoá vào Thùng rác: dùng API Trash của macOS (qua `osascript` Finder hoặc `NSFileManager`) để chạy cả với ổ ngoài.
- Trang Kiểm tra máy + nút "Xuất gói chẩn đoán" (log, phiên bản, cấu hình đã che khoá).
- Giữ quyền agent "Toàn quyền" (agent cần chạy script của skill); chấp nhận vì máy cá nhân, ghi rõ rủi ro nội dung bài mẫu/yêu cầu lạ trong tài liệu.

### 3. Mọi thể loại

- `project.json` thêm `genre` (gợi ý từ bài mẫu hoặc người dùng chọn) và `language` (mặc định `vi`).
- Mẫu lời dặn theo thể loại (pop, ballad, bolero, rock, rap, EDM, thiếu nhi, quê hương…) thay cho danh sách thuần quê hương.
- Skill: Styles và thẻ đoạn sinh từ số đo bài mẫu cho mọi thể loại; bỏ ràng buộc "TH Music"; profile A/B thành ví dụ tham khảo.
- Lục bát: chỉ hiện khi lời có dạng lục bát; kiểm tra chung thay bằng đếm chữ/câu và vần cuối.
- Video: giao diện (font, màu tô karaoke, kiểu tựa đề) theo thể loại; font có đủ dấu tiếng Việt; tách token theo ký tự cho ngôn ngữ không có dấu cách (để mở đường, không bắt buộc lần này).
- Đổi tên hiển thị? (xem câu hỏi mở).

### 4. LLM trực tiếp

- Ba việc (soát lời, phối khí, lên cảnh) thành một lời gọi LLM trả JSON theo schema; Go kiểm tra và ghi file. Nhanh hơn, rẻ hơn, không cần cài Claude/Codex, không chạy lệnh shell từ nội dung người dùng.
- Agent CLI giữ cho "Giao việc cho agent" và "Điền form Suno" (tuỳ chọn, tắt mặc định vì điều khoản Suno).

### 5. Chất lượng phân tích (vòng lặp đo được)

- Bộ thử: 6 bài (tiêu chí 4) với timing kiểm tay cho 3 câu mỗi bài.
- Chỉ số: lệch đầu câu trung bình (ms), % chữ chia đều, % khớp cao độ bản nhạc, % ô nhịp có phách mạnh đúng.
- Mỗi lần đổi một thành phần, giữ khi chỉ số tốt hơn và không phá bài khác. Lưu ý giấy phép: trọng số Demucs và nhiều model canh lời là phi thương mại.

### 6. Nhiều nguồn tạo nhạc

- `gen/spec.json` trung lập (tempo, giọng, nhịp, nhạc cụ, giọng hát, đường năng lượng, thẻ đoạn); Go dịch sang từng nhà cung cấp; mỗi lần tạo là một thư mục bất biến (yêu cầu, phản hồi, model, seed, gói Suno lúc tạo).
- Trước khi làm: nghe thử 1 bài tiếng Việt trên Suno v6, ElevenLabs, ACE-Step.

### 7. Đóng gói

- Chỉ dùng trên máy bạn và vài người quen: .app không ký + hướng dẫn "Vẫn mở".
- Phát rộng: Developer ID ($99/năm), DMG, notarize, ffmpeg bản LGPL dùng `h264_videotoolbox`.

## Rủi ro

- Giấy phép model: không chặn vì chỉ dùng riêng; xem lại nếu sau này kiếm tiền.
- Chất lượng tiếng Việt của nguồn nhạc ngoài Suno chưa được kiểm.
- Suno giới hạn số lượt tải mỗi tháng theo gói; điền form tự động là vùng xám.

## Câu hỏi mở

Xem phần cuối báo cáo trả lời người dùng; giai đoạn 1–3 có thể làm ngay khi được duyệt.

## Kết quả giai đoạn 1–3

- Hub ảnh nhận yêu cầu 2048x1152 nhưng trả 1672x941 (16:9); khung dựng giờ lấy mẫu một lần từ ảnh gốc (độ nét +9%, ~7 ms/khung).
- Gemini làm theo tài liệu, chưa chạy với khoá thật. Chuyển bài sang ổ ngoài (Finder) chưa thử trên ổ thật.
- Tiếng Trung/Nhật canh và tô chữ theo cả câu, chưa theo từng chữ.
