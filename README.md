# DVPL

# DVPL

Công cụ Go thuần để nén/giải nén DVPL, gồm CLI và TUI Bubble Tea trong cùng binary. Cài Go 1.22+ hoặc tải binary phù hợp trong `dist/`.

| Nền tảng | Binary |
|---|---|
| Linux x86-64 | `dvpl-linux-amd64` |
| Linux ARM64 | `dvpl-linux-arm64` |
| Windows 64-bit | `dvpl-windows-amd64.exe` |
| Windows 32-bit | `dvpl-windows-386.exe` |

Build đủ bản bằng `make build VERSION=1.0.0`; tạo SHA256 bằng `make checksums VERSION=1.0.0`. Tất cả bản phát hành đặt `CGO_ENABLED=0`. Bản purego tùy chọn: `CGO_ENABLED=0 go build -tags purego ./cmd/dvpl`.

Windows: SmartScreen có thể cảnh báo binary chưa ký. Chạy từ Windows Terminal hoặc PowerShell để dùng TUI; tiếng Việt cần terminal hỗ trợ UTF-8.

```sh
dvpl compress -level 2 assets/
dvpl decompress assets.dvpl
dvpl c -o output/ -filter '*.png,*.json' assets/
dvpl input.txt # tự nén; file có đuôi .dvpl được tự giải nén
dvpl tui      # giao diện tương tác trên terminal
```

Không truyền tham số trong terminal tương tác cũng mở TUI. Giao diện có form nén/giải nén, trình duyệt đường dẫn, tiến trình có ETA và màn hình lỗi. Bảng phím:

| Phím | Form | Trình duyệt | Tiến trình / Kết quả |
|---|---|---|---|
| `Tab` / `↓` | Chuyển đến trường tiếp theo | Chọn mục tiếp theo | — |
| `Shift+Tab` / `↑` | Chuyển về trường trước | Chọn mục trước | Cuộn lỗi lên |
| `←` / `→` | Đổi Mode, Level hoặc Workers tùy trường | — | — |
| `+` / `-` | Tăng/giảm Level hoặc Workers | — | — |
| `Space` | Bật/tắt tùy chọn | Chọn thư mục hiện tại | — |
| `Enter` | Chọn trường hoặc bắt đầu | Vào thư mục / chọn tệp | Kết quả: quay lại form |
| `Backspace` | — | Lên thư mục cha; ở gốc Windows hiện danh sách ổ đĩa | — |
| `Esc` | Lùi focus | Đóng trình duyệt | Tiến trình: hủy an toàn; kết quả: về form |
| `L` | Đổi tiếng Việt/Anh | — | — |
| `PgUp` / `PgDn` | — | — | Cuộn danh sách lỗi |
| `s` | — | — | Kết quả: lưu `errors.log` |
| `q` / `Ctrl+C` | Thoát | Đóng TUI | Thoát; tác vụ đang chạy được hủy |

TUI có các tùy chọn giữ/xóa file gốc, force và trust-CRC. Màn hình tiến trình hiển thị thanh tiến độ, số file, tốc độ, thời gian đã chạy và ETA. Khung lỗi giữ 500 lỗi gần nhất; màn hình kết quả hiển thị danh sách đầy đủ. Giao diện mặc định tiếng Việt; dùng `dvpl tui --lang en`, `DVPL_LANG=en` hoặc nhấn `L` để chọn tiếng Anh. Các lệnh `c`/`d` và `compress`/`decompress` vẫn dùng được như trước.

Để thêm ngôn ngữ, tạo `internal/tui/lang/<mã>.json` với cùng tập key như `vi.json` và `en.json`. Các file JSON được nhúng tự động. Thêm mã ngôn ngữ vào `lang.Set`/`lang.Other` và phần kiểm tra `--lang`/`DVPL_LANG` trong CLI; cập nhật test đối chiếu key để kiểm tra file mới.

| Cờ | Mặc định | Ý nghĩa |
|---|---:|---|
| `-o` | cạnh input | Thư mục đầu ra; muốn tên file khác thì đổi tên sau khi xử lý |
| `-level` | `2` | `0` không nén, `1` LZ4, `2` LZ4HC |
| `-f` | tắt | Ép nén |
| `-filter`, `-ignore`, `-ignore-compress` | — | Mẫu tên file, phân cách bằng dấu phẩy |
| `-workers` | tự chọn | Số worker |
| `-trust-crc` | tắt | Bỏ kiểm tra CRC (có cảnh báo) |
| `-delete-original` | tắt | Xóa file gốc sau khi xử lý thành công |
| `-q`, `-v` | tắt | Im lặng theo file / hiện chi tiết byte |

| Biến môi trường | Ý nghĩa |
|---|---|
| `DVPL_LEVEL` | Mức nén `0`–`2`; cờ `-level` được ưu tiên |
| `DVPL_WORKERS` | Số worker dương; cờ `-workers` được ưu tiên |

Khi giải nén, type `0` là dữ liệu thô, type `1` và `2` là khối LZ4, type `4` là một khối LZ4 có header 28 byte. Type `4` bỏ qua CRC của payload và hiện chỉ hỗ trợ đọc; chương trình không tạo file type `4`.

Mã thoát: `0` thành công, `1` có file lỗi, `2` lỗi cấu hình/đối số, `130` khi bị ngắt. Mặc định chương trình giữ file gốc. Symlink bị bỏ qua. Nhãn type trong footer chưa được xác minh bằng file game thật.
