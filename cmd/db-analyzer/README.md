# db-analyzer

Công cụ dòng lệnh phân tích 1 collection MongoDB để phục vụ việc thiết kế index. Chạy xong, bạn có 1 thư mục file JSON: mỗi file trả lời 1 câu hỏi về collection, ví dụ collection đang có index nào, hay query của ứng dụng có dùng đúng index không. Tool chỉ đọc: không tạo/xóa index, không sửa dữ liệu.

## Chạy lần đầu

Mọi lệnh chạy từ thư mục gốc repo.

**1. Cài đặt:** Go 1.27+, Make, và [`mongosh`](https://www.mongodb.com/docs/mongodb-shell/install/) gọi được từ dòng lệnh. Các phân tích được chạy bằng `mongosh`.

**2. Chọn database cần phân tích.** Mở `cmd/db-analyzer/config.yaml`, sửa `mongo.database` và `mongo.collection` thành database và collection của bạn. URI kết nối có mật khẩu thì không để trong file này, mà đặt trong `.env`:

```bash
cp cmd/db-analyzer/.env.example cmd/db-analyzer/.env
# sửa dòng MONGO_URI trong cmd/db-analyzer/.env
```

**3. Kiểm tra kết nối:**

```bash
make db-analyzer-check-connection
```

Thấy dòng `connected to database` là kết nối được và collection có tồn tại.

**4. Chạy phân tích:**

```bash
make db-analyzer-inspect    # phân tích chung: chạy được với mọi collection
make db-analyzer-collect    # phân tích chung + phân tích riêng của collection
```

`collect` chỉ chạy được với collection đã có bộ phân tích riêng, hiện có `threads_posts`. Collection khác thì `collect` báo lỗi, khi đó dùng `inspect`. Với collection lớn, `collect` có thể mất vài phút, vì có đếm và explain trên dữ liệu thật; nên chạy lúc tải thấp nếu đó là database production.

**5. Xem kết quả.** Dòng log cuối cho biết thư mục kết quả:

```text
INF run complete failures=0 output_dir=output/testing/20260925T090605.899204840Z scripts=5
```

Trong thư mục đó:

- **`manifest.json`:** danh sách phân tích đã chạy, mỗi cái `status` là `ok` hoặc `error` kèm lý do. Mở file này trước.
- **Mỗi phân tích 1 file JSON**, kết quả nằm trong field `output_json`. Xem bảng dưới để biết file nào trả lời câu hỏi gì.

## Mỗi file kết quả cho biết gì

| File | Trả lời câu hỏi | Xem ở đâu trong `output_json` |
| --- | --- | --- |
| `common__collection_stats.json` | Collection lớn cỡ nào? | `estimated_document_count`, `data_size_bytes`, `total_index_size_bytes` |
| `common__indexes.json` | Collection đang có những index nào? | `indexes`: tên và các field của từng index |
| `collections__<collection>__schema_sample.json` | Dữ liệu thực tế trông ra sao? | `field_types`: mỗi field thiếu ở bao nhiêu record, mang kiểu gì (trên 1 mẫu) |
| `collections__<collection>__filter_selectivity.json` | Mỗi điều kiện lọc của ứng dụng khớp bao nhiêu record? | `results`: `count` của từng điều kiện; số càng nhỏ so với tổng thì điều kiện càng nên nằm trong index |
| `collections__<collection>__workload_explain.json` | Query của ứng dụng dùng index nào, có hiệu quả không? | `results`: `indexes_used`, và trong `explain.executionStats` so `totalDocsExamined` với `nReturned`; đọc nhiều hơn trả về rất nhiều là query cần index tốt hơn |

2 file đầu có ở cả `inspect` lẫn `collect`. 3 file sau chỉ có ở `collect`.

Một phân tích có `status: ok` vẫn có thể có lỗi ở từng phép đo bên trong, ví dụ 1 điều kiện đếm quá thời gian, ghi trong field `error` của phép đo đó.

## Các lệnh

| Lệnh | Làm gì |
| --- | --- |
| `make db-analyzer-check-connection` | Kết nối, xác nhận collection tồn tại. |
| `make db-analyzer-list` | In tên các phân tích có sẵn. |
| `make db-analyzer-inspect` | Chạy phân tích chung. |
| `make db-analyzer-collect` | Chạy phân tích chung, rồi phân tích riêng của collection. |
| `make db-analyzer-run SCRIPT=<tên>` | Chạy đúng 1 phân tích, tên lấy từ `list`. |

Muốn dùng file cấu hình khác thì thêm `CONFIG_FILE=<đường dẫn>` hoặc `ENV_FILE=<đường dẫn>` vào lệnh `make`.

Mã thoát: `0` nếu mọi phân tích `ok`; `1` nếu có phân tích lỗi hoặc lỗi cấu hình/kết nối; `130` nếu bị dừng bằng Ctrl-C trước khi xong, `143` nếu bị dừng bằng SIGTERM, ví dụ `docker stop`. Khi có phân tích lỗi, kết quả của các phân tích khác vẫn được ghi.

## Cấu hình

Cách các nguồn cấu hình đè lên nhau (`config.yaml`, `.env`, biến môi trường): xem [README gốc](../../README.md#cấu-hình).

| Key trong `config.yaml` | Tên biến (`.env`) | Mặc định trong code | Ý nghĩa |
| --- | --- | --- | --- |
| `mongo.uri` | `MONGO_URI` | `mongodb://localhost:27017` | URI kết nối. Mật khẩu có ký tự đặc biệt (ví dụ `$`) phải percent-encode. |
| `mongo.database` | `MONGO_DATABASE` | `test` | Database cần phân tích. |
| `mongo.collection` | `MONGO_COLLECTION` | `threads_posts` | Collection cần phân tích; quyết định `collect` chạy bộ phân tích riêng nào. |
| `outputDir` | `OUTPUT_DIR` | `output` | Nơi ghi kết quả. Đường dẫn tương đối tính từ thư mục chứa `config.yaml`; `config.yaml` đặt `../../output/testing`, tức `output/testing/` ở gốc repo. |
| `queryTimeout` | `QUERY_TIMEOUT` | `20s` | Thời gian tối đa cho bước kiểm tra kết nối và cho từng query trong các phân tích riêng. |
| `scriptTimeout` | `SCRIPT_TIMEOUT` | `3m` | Thời gian tối đa cho 1 phân tích. |
| `sampleSize` | `SAMPLE_SIZE` | `500` | Số record lấy mẫu cho `schema_sample`. |
| `explainLimit` | `EXPLAIN_LIMIT` | `100` | Số kết quả tối đa mỗi query khi explain. |
| `appEnv` | `APP_ENV` | `dev` | `dev`/`testing`: log dễ đọc, có debug. `staging`/`production`: log JSON. |

## Gọi thẳng binary

`make` build ra `bin/db-analyzer`. Gọi thẳng khi cần:

| Flag | Mặc định | Ý nghĩa |
| --- | --- | --- |
| `-action` | `inspect` | `check`, `list`, `inspect`, `collect`, `run`. |
| `-script` | | Tên phân tích cho `-action run`. |
| `-config` | `config.yaml` | File YAML cấu hình. Để rỗng (`-config=`) thì không dùng. |
| `-env-file` | `.env` | File `.env`. Không có file thì bỏ qua; để rỗng (`-env-file=`) thì không đọc. |
| `-scripts-dir` | | Đọc script từ thư mục trên đĩa thay vì bản có sẵn trong binary, dùng khi đang sửa script: `-scripts-dir services/mongoanalyzer/scripts`. |

## Chạy bằng Docker

Không cần cài Go hay `mongosh`, image đã có sẵn:

```bash
make db-analyzer-image
docker run --rm --env-file cmd/db-analyzer/.env db-analyzer -action check
docker run --rm --env-file cmd/db-analyzer/.env -v "$PWD/out:/out" db-analyzer -action collect
```

Image dùng `config.yaml` của deployment. Biến truyền bằng `--env-file`/`-e` đè lên. Kết quả ghi vào `/out` trong container, nên mount 1 thư mục vào đó, và thư mục này phải cho user trong container (uid 65532) ghi được.

> Image chưa được build thử lần nào, vì máy phát triển hiện tại chưa có quyền Docker.

## Thêm phân tích mới

Xem [README của mongoanalyzer](../../services/mongoanalyzer/README.md).
