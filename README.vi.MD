# DVPL

DVPL là công cụ dòng lệnh viết bằng Go và giao diện terminal để nén, giải nén file `.dvpl` dùng trong game chạy trên Dava Engine, gồm World of Tanks Blitz. Chương trình hỗ trợ Linux amd64/arm64 và Windows amd64/386, không cần CGO. Giao diện có tiếng Việt và tiếng Anh.

English: [README.md](README.md).

## Cài đặt

### Arch Linux

Từ mã nguồn dự án, chạy:

```sh
cd packaging/arch
makepkg -si
```

### Binary Linux và Windows

Tải archive phù hợp với hệ điều hành và CPU từ [GitHub Releases](https://github.com/0rangeC4t/dvpl/releases/latest), rồi giải nén. Archive có chương trình, hai file README, `LICENSE` và `NOTICE`. Chạy `dvpl` trên Linux hoặc `dvpl.exe` trên Windows; có thể thêm thư mục đã giải nén vào `PATH` để gọi lệnh từ mọi thư mục.

| Nền tảng | Archive |
|---|---|
| Linux x86-64 | `dvpl-<version>-linux-amd64.tar.gz` |
| Linux ARM64 | `dvpl-<version>-linux-arm64.tar.gz` |
| Linux x86-64, purego | `dvpl-<version>-linux-amd64-purego.tar.gz` |
| Windows 64-bit | `dvpl-<version>-windows-amd64.zip` |
| Windows 32-bit | `dvpl-<version>-windows-386.zip` |

Windows có thể hiện cảnh báo SmartScreen vì chương trình chưa được ký số. Chạy từ Windows Terminal hoặc PowerShell. TUI tiếng Việt cần terminal hỗ trợ UTF-8.

## Dùng CLI

```sh
dvpl compress -level 2 assets/             # nén cả thư mục
dvpl decompress assets.dvpl                # giải nén một file
dvpl c -o output/ -filter '*.png,*.json' assets/
dvpl input.txt                             # nén; input .dvpl sẽ được giải nén
dvpl --version
```

`c`/`d` và `encrypt`/`decrypt` là bí danh của `compress`/`decompress`. Không ghi lệnh thì input đuôi `.dvpl` được giải nén, input khác được nén. Cờ phải đứng trước đường dẫn. Thư mục input được duyệt đệ quy; khi dùng `-o`, cấu trúc thư mục con được giữ nguyên. Nếu không có `-o`, file đầu ra nằm cạnh input. File đầu ra đã tồn tại sẽ được thay thế. File được ghi vào file tạm cùng thư mục rồi đổi tên. Symlink và file tạm `.dvpltmp` được bỏ qua. Chương trình cảnh báo nếu thư mục đầu ra nằm bên trong thư mục đầu vào. File gốc được giữ lại trừ khi dùng `-delete-original`.

| Cờ | Mặc định | Ý nghĩa |
|---|---:|---|
| `-o` | cạnh input | Thư mục đầu ra |
| `-level` | `2` | Mức nén: `0` lưu nguyên dạng, `1` LZ4, `2` LZ4HC |
| `-f` | tắt | Giữ kiểu nén đã chọn kể cả khi đầu ra không nhỏ hơn |
| `-filter` | — | Chỉ xử lý tên file khớp mẫu `filepath.Match`, phân cách bằng dấu phẩy |
| `-ignore` | — | Bỏ qua tên file khớp mẫu |
| `-ignore-compress` | — | Lưu nguyên dạng các file khớp mẫu |
| `-workers` | `2` | Số worker, tối đa bằng số CPU khả dụng |
| `-trust-crc` | tắt | Bỏ so sánh CRC payload khi giải nén |
| `-delete-original` | tắt | Chỉ xóa file nguồn sau khi chuyển đổi thành công |
| `-q` | tắt | Ẩn thông báo từng file |
| `-v` | tắt | Hiện số byte và đường dẫn đầu ra cho từng file |

`DVPL_LEVEL` và `DVPL_WORKERS` đặt giá trị mặc định nếu không truyền cờ tương ứng. `DVPL_LANG` chọn ngôn ngữ TUI (`vi` hoặc `en`). Mã thoát: `0` thành công, `1` có một hay nhiều file lỗi, `2` lỗi đối số hoặc tác vụ, `130` khi bị ngắt. Khi giải nén, chương trình kiểm tra magic DVPL, kích thước và CRC payload, trừ khi `-trust-crc` bỏ qua so sánh CRC. Một file lỗi không làm dừng các file còn lại.

Định dạng có footer little-endian 20 byte gồm kích thước gốc, kích thước payload, CRC32-IEEE của payload, type và magic `DVPL`. Khi đọc, chương trình chấp nhận type `0` (dữ liệu thô), `1` và `2` (khối LZ4), `4` (một khối LZ4 có header 28 byte; không kiểm tra CRC payload). Chương trình ghi được type `0`–`2`; type `4` chỉ đọc. Type khác bị từ chối. Nếu nén không nhỏ hơn, đầu ra dùng type `0`, trừ khi có `-f`. Kích thước tối đa là `0x7E000000` byte. Nhãn type chưa được xác minh bằng file game thật.

## Dùng TUI

Chạy `dvpl tui` để mở giao diện. Chạy `dvpl` không tham số sẽ mở TUI khi standard input là terminal tương tác. Dùng trình duyệt để chọn file hoặc thư mục nguồn, chọn thư mục đích, mức nén và số worker, rồi chọn **Bắt đầu**. Màn hình tiến trình hiển thị số liệu, tốc độ, thời gian đã chạy và thời gian còn lại ước tính. Khi đang chạy, khung lỗi giữ 500 lỗi gần nhất; màn hình kết quả hiện toàn bộ lỗi. Nhấn `s` tại màn hình kết quả để lưu lỗi vào `errors.log`.Trong TUI, số worker mặc định bằng số luồng CPU (1–64) và lựa chọn lần trước được nhớ cho lần chạy sau.

Dùng `dvpl tui --lang en`, `dvpl --lang en` trong terminal tương tác, `DVPL_LANG=en` hoặc phím `L` trong form để chọn tiếng Anh. Mặc định là tiếng Việt.

| Phím | Form | Trình duyệt | Tiến trình / Kết quả |
|---|---|---|---|
| `Tab` / `↓` | Chuyển đến trường tiếp theo | Chọn mục bên dưới | Cuộn lỗi xuống |
| `Shift+Tab` / `↑` | Quay về trường trước | Chọn mục bên trên | Cuộn lỗi lên |
| `←` / `→` | Đổi chế độ, mức nén hoặc số worker theo trường đang chọn | — | — |
| `+` / `-` | Tăng/giảm mức nén hoặc số worker đang chọn | — | — |
| `Space` | Đổi chế độ hoặc bật/tắt tùy chọn đang chọn | Chọn thư mục hiện tại | — |
| `Enter` | Chọn, mở trình duyệt hoặc bắt đầu tác vụ | Mở thư mục hoặc chọn file | Kết quả: về form |
| `Backspace` | — | Lên thư mục cha; tại gốc ổ Windows sẽ hiện danh sách ổ | — |
| `Esc` | Lùi focus | Đóng trình duyệt | Tiến trình: hủy; kết quả: về form |
| `L` | Đổi Việt/Anh khi không nhập văn bản | — | — |
| `PgUp` / `PgDn` | — | — | Cuộn danh sách lỗi |
| `s` | — | Chọn thư mục hiện tại | Kết quả: lưu `errors.log` |
| `q` / `Ctrl+C` | Thoát (trừ `q` khi đang nhập văn bản) | — | Thoát |

## Build từ mã nguồn

Cần Go 1.22 trở lên và Make. Tại thư mục gốc:

```sh
make build VERSION=0.1.0
```

Lệnh tạo binary Linux amd64/arm64, Windows amd64/386 và Linux amd64 purego trong `dist/`. Trước khi build binary Windows cho một phiên bản, tạo lại resource icon và version bằng `make winres VERSION=0.1.0` (cần `llvm-rc` và `llvm-cvtres`). `make checksums VERSION=0.1.0` build cả năm binary và ghi checksum của chúng vào `dist/SHA256SUMS`.

## Giấy phép

DVPL dùng giấy phép Apache-2.0; xem `LICENSE` và `NOTICE`. Dự án tham khảo qirashi/dvpl_go (Apache-2.0) và Tankerch/dvpl_converter (MIT). Dependency gồm [pierrec/lz4](https://github.com/pierrec/lz4) (BSD-3-Clause) và Bubble Tea, Bubbles, Lip Gloss của Charmbracelet (MIT). Đây là công cụ cộng đồng không chính thức, không liên kết với Wargaming.
