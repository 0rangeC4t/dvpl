# DVPL

Công cụ dòng lệnh Go để nén và giải nén file DVPL. Cài Go 1.22 trở lên, sau đó chạy `go build ./cmd/dvpl` hoặc dùng binary trong `dist/`.

```sh
dvpl compress -level 2 assets/
dvpl decompress assets.dvpl
dvpl c -o output/ -filter '*.png,*.json' assets/
dvpl input.txt # tự nén; file có đuôi .dvpl được tự giải nén
dvpl tui      # giao diện tương tác trên terminal
```

Không truyền tham số trong terminal tương tác cũng mở TUI. TUI cho chọn nén/giải nén, nhập nguồn và thư mục đích, đặt level `0/1/2` bằng trái/phải, `+/-` hoặc gõ số. Workers mặc định theo số CPU và chọn được từ `1` đến `64` bằng cách gõ số hoặc `+/-`. Các tùy chọn giữ/xóa file gốc, force và trust-crc cũng có trong form. Màn hình tiến trình không in từng file thành công; thanh tiến độ, bộ đếm, tốc độ và thời gian được cập nhật trong giao diện. Khung lỗi cuộn được giữ 500 lỗi gần nhất; màn hình kết quả có tổng byte và toàn bộ danh sách lỗi cuộn được. Nhấn `s` ở màn hình kết quả để lưu danh sách vào `errors.log`. Giao diện mặc định tiếng Việt; chạy `dvpl tui --lang en`, đặt `DVPL_LANG=en`, hoặc nhấn `L` trong form để đổi tiếng Anh. Dùng Tab/mũi tên để di chuyển, Enter để chọn, Esc để quay lại/hủy tác vụ, `q` hoặc Ctrl+C để thoát. Các lệnh `c`/`d` và `compress`/`decompress` vẫn dùng được như trước.

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
