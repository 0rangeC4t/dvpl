# Gói Arch Linux

Build gói trong thư mục này bằng `makepkg`.

- Build và cài: `makepkg -si`
- Gỡ cài đặt: `sudo pacman -R dvpl-git`

Gói được build từ GitHub và cần `git`, `go` để build. `check()` chạy `go test ./...`.
