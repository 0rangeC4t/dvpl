# DVPL

Công cụ dòng lệnh Go để nén và giải nén file DVPL. Cài Go 1.22 trở lên, sau đó chạy `go build ./cmd/dvpl` hoặc dùng binary trong `dist/`.

```sh
dvpl compress -level 2 assets/
dvpl decompress assets.dvpl
dvpl c -o output/ -filter '*.png,*.json' assets/
dvpl input.txt # tự nén; file có đuôi .dvpl được tự giải nén
```

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

Mã thoát: `0` thành công, `1` có file lỗi, `2` lỗi cấu hình/đối số, `130` khi bị ngắt. Mặc định chương trình giữ file gốc. Symlink bị bỏ qua. Nhãn type trong footer chưa được xác minh bằng file game thật.
