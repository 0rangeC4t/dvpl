# DVPL

Công cụ dòng lệnh Go để nén và giải nén file `.dvpl` (định dạng của Dava Engine, dùng trong World of Tanks Blitz), kèm giao diện terminal (TUI). Tiếng Anh: [README.en.md](README.en.md).

## Cài đặt

Cần Go 1.22 trở lên.

```sh
git clone https://github.com/0rangeC4t/dvpl.git
cd dvpl
go build -o dvpl ./cmd/dvpl     # bản cho máy hiện tại
make build                      # nhiều nền tảng, kết quả trong dist/
```

`make build` tạo `dist/dvpl-linux-amd64`, `dist/dvpl-linux-arm64`, `dist/dvpl-windows-amd64.exe` và `dist/dvpl-linux-amd64-purego`.

Hiện cả hai chế độ build (`cgo` và `purego`) đều dùng cùng một backend thuần Go (`pierrec/lz4`). Backend cgo riêng mới nằm trong kế hoạch.

## Sử dụng

```sh
dvpl compress -level 2 assets/          # nén cả thư mục
dvpl decompress assets.dvpl             # giải nén một file
dvpl c -o output/ -filter '*.png,*.json' assets/
dvpl input.txt                          # tự nén; file đuôi .dvpl tự giải nén
dvpl tui                                # giao diện terminal
```

Chạy `dvpl` không tham số trong terminal tương tác cũng mở TUI. Các lệnh `c`/`d` và `compress`/`decompress` (cùng bí danh `encrypt`/`decrypt`) dùng được như nhau. **Các cờ phải đứng trước đường dẫn.**

| Cờ | Mặc định | Ý nghĩa |
|---|---:|---|
| `-o` | cạnh input | Thư mục đầu ra (giữ nguyên cấu trúc thư mục con) |
| `-level` | `2` | `0` không nén, `1` LZ4, `2` LZ4HC |
| `-f` | tắt | Giữ kiểu nén đã chọn kể cả khi kết quả không nhỏ hơn bản gốc |
| `-filter` | — | Chỉ xử lý file có tên khớp mẫu (phân cách bằng dấu phẩy) |
| `-ignore` | — | Bỏ qua file có tên khớp mẫu |
| `-ignore-compress` | — | Lưu không nén các file khớp mẫu |
| `-workers` | `2` | Số worker, tối đa bằng số CPU |
| `-trust-crc` | tắt | Bỏ kiểm tra CRC khi giải nén (có cảnh báo) |
| `-delete-original` | tắt | Xóa file gốc sau khi xử lý thành công |
| `-q`, `-v` | tắt | Im lặng theo file / hiện chi tiết byte |

Mẫu so khớp với **tên file** (không phải đường dẫn), theo cú pháp `filepath.Match`. File đầu ra đã tồn tại sẽ bị ghi đè.

| Biến môi trường | Ý nghĩa |
|---|---|
| `DVPL_LEVEL` | Mức nén `0`–`2`; cờ `-level` được ưu tiên |
| `DVPL_WORKERS` | Số worker dương; cờ `-workers` được ưu tiên |
| `DVPL_LANG` | Ngôn ngữ TUI: `vi` hoặc `en` |

Mã thoát: `0` thành công, `1` có file lỗi, `2` lỗi cấu hình/đối số, `130` khi bị ngắt. Một file lỗi không làm dừng cả lô.

## An toàn dữ liệu

- Mặc định **giữ file gốc**; chỉ xóa khi có `-delete-original`.
- Ghi file qua file tạm `*.dvpltmp` cùng thư mục rồi đổi tên, nên file đầu ra không bị ghi dở.
- Symlink bị bỏ qua. Cảnh báo nếu thư mục đầu ra nằm trong thư mục đầu vào.
- Kiểm tra magic, kích thước và CRC32 của payload khi giải nén.

## TUI

**Cách mở:**

```sh
dvpl                  # không tham số, trong terminal tương tác
dvpl tui              # luôn mở TUI
dvpl tui --lang en    # mở bằng tiếng Anh
dvpl --lang en        # cũng mở TUI (khi chạy trong terminal tương tác)
./dist/dvpl-linux-amd64 tui    # nếu chạy bản build trong dist/
```

`dvpl` không tham số chỉ mở TUI khi đầu vào là terminal thật; nếu chạy trong pipe hoặc script, tool báo lỗi thiếu đường dẫn (mã thoát `2`), nên dùng `dvpl tui` để chắc chắn.

Chọn nén/giải nén, nhập nguồn và thư mục đích, đặt mức nén `0/1/2` (trái/phải, `+/-` hoặc gõ số), số worker (mặc định bằng số CPU, nhập được 1 đến 64 nhưng thực tế bị giới hạn bởi số CPU), và các tùy chọn giữ/xóa file gốc, force, trust-crc. Màn hình tiến trình hiển thị thanh tiến độ, bộ đếm, tốc độ và thời gian; khung lỗi cuộn được giữ 500 lỗi gần nhất. Ở màn hình kết quả, nhấn `s` để lưu danh sách lỗi vào `errors.log`.

Giao diện mặc định tiếng Việt. Đổi bằng `dvpl tui --lang en`, `DVPL_LANG=en` hoặc phím `L` trong form. Dùng Tab/mũi tên để di chuyển, Enter để chọn, Esc để quay lại/hủy, `q` hoặc Ctrl+C để thoát.

Thêm ngôn ngữ: tạo `internal/tui/lang/<mã>.json` với cùng tập key như `vi.json` và `en.json` (file JSON được nhúng tự động), thêm mã vào `lang.Set`/`lang.Other` và phần kiểm tra `--lang`/`DVPL_LANG` trong CLI, rồi cập nhật test đối chiếu key.

## Định dạng DVPL

```
[payload (nén hoặc nguyên bản)][footer 20 byte, little-endian]

footer:
  uint32  kích thước gốc
  uint32  kích thước payload
  uint32  CRC32 (IEEE) của payload
  uint32  kiểu nén
  4 byte  "DVPL"
```

| Type | Ý nghĩa | Đọc | Ghi |
|---:|---|:--:|:--:|
| `0` | không nén | có | có |
| `1` | khối LZ4 | có | có |
| `2` | khối LZ4HC | có | có |
| `4` | một khối LZ4 có header 28 byte, bỏ qua CRC của payload | có | không |

Giá trị khác bị từ chối. Nếu nén không làm file nhỏ hơn (và không dùng `-f`), tool tự ghi type `0`. Giới hạn kích thước là `0x7E000000` byte. Nhãn type trong footer chưa được xác minh bằng file game thật.

## Kiểm thử

```sh
go test ./...
go test -tags purego ./...
```

## Giấy phép và ghi công

Apache-2.0, xem `LICENSE` và `NOTICE`. Dự án tham khảo [qirashi/dvpl_go](https://github.com/qirashi/dvpl_go) (Apache-2.0) và [Tankerch/dvpl_converter](https://github.com/Tankerch/dvpl_converter) (MIT) làm tài liệu tham khảo, không sao chép mã nguồn. Phụ thuộc: [pierrec/lz4](https://github.com/pierrec/lz4) (BSD-3-Clause), Bubble Tea, Bubbles và Lip Gloss của Charmbracelet (MIT).

Đây là công cụ cộng đồng không chính thức
                                                                         , không liên kết với Wargaming.
