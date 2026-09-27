# Backlog

Các vấn đề còn tồn đọng của project, xếp theo mức độ ưu tiên. Mỗi mục gồm: vấn đề là gì, ảnh hưởng thế nào, và hướng xử lý. Mục nào xử lý xong thì xóa khỏi file này.

## Cao

### Thiếu test tự động

- **Vấn đề:** chỉ `core/config` và `core/app` có test. Các phần còn lại phải kiểm tra bằng tay với MongoDB thật.
- **Ảnh hưởng:** sửa code dễ làm hỏng hành vi cũ mà không ai biết.
- **Hướng xử lý:** viết unit test cho các phần không cần DB:
  - `jsrunner/catalog.go`: đọc plan, validate tên script, kiểm tra script có đúng collection không.
  - `jsrunner/runner.go`: tách dòng `ANALYZER_RESULT_JSON:` và `ANALYZER_PROGRESS_JSON:` khỏi output của `mongosh`, che URI trong output và lỗi.
  - `transport/cli`: dispatch action, thông báo lỗi khi action sai hoặc script không tồn tại.

## Trung bình

### Image Docker chưa được build thử

- **Vấn đề:** `cmd/db-analyzer/Dockerfile` chưa được build lần nào, vì máy phát triển hiện tại chưa có quyền dùng Docker (user chưa thuộc group `docker`).
- **Ảnh hưởng:** luồng deploy bằng image chưa được kiểm chứng: cài `mongosh`, chạy bằng user không phải root, đọc cấu hình từ biến môi trường.
- **Hướng xử lý:** cấp quyền Docker, rồi chạy `make db-analyzer-image` và `docker run --env-file cmd/db-analyzer/.env db-analyzer -action check`. Xác nhận image không chứa `.env`.

## Thấp

### Thông báo lỗi trỏ tới lệnh không tồn tại

- **Vấn đề:** khi `-action run` với script không tồn tại, lỗi in ra là `use make list to see available scripts`. Target đúng là `make db-analyzer-list`.
- **Hướng xử lý:** sửa thông báo trong `services/mongoanalyzer/transport/cli/cli.go`. Tốt hơn nữa là không nhắc tới `make`, vì transport không biết deployment chạy bằng gì.
