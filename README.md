# github.com/kienvan102/monorepo-microservices-template

Tập hợp công cụ nội bộ viết bằng Go, tổ chức dạng monorepo: mỗi công cụ build và deploy độc lập.

## Có gì chạy được

| Công cụ | Dùng để làm gì |
| --- | --- |
| [**db-analyzer**](cmd/db-analyzer/README.md) | Thu thập số liệu từ một collection MongoDB để phục vụ thiết kế index. Chỉ đọc, không sửa dữ liệu. |

Muốn chạy 1 công cụ: mở README của nó, làm theo mục "Chạy lần đầu".

## Cấu hình

Mỗi công cụ đọc cấu hình từ 4 tầng. Giá trị được lấy từ tầng cao nhất có khai nó:

1. Biến môi trường thật (`export`, `docker run -e`/`--env-file`).
2. File `.env` của công cụ (`cmd/<tên>/.env`). Không có file thì bỏ qua.
3. File YAML của công cụ (`cmd/<tên>/config.yaml`).
4. Giá trị mặc định trong code.

Thường thì `config.yaml` chứa cấu hình chạy được ngay, còn `.env` chỉ khai những giá trị cần đè trên máy mình, ví dụ URI có mật khẩu.

Key trong YAML được đổi sang tên biến như sau: key lồng nhau nối bằng `_`, mỗi chữ hoa bắt đầu 1 từ mới, rồi viết hoa toàn bộ. Ví dụ:

| Key trong YAML | Tên biến |
| --- | --- |
| `mongo.uri` | `MONGO_URI` |
| `queryTimeout` | `QUERY_TIMEOUT` |
| `query_timeout` | `QUERY_TIMEOUT` |
| `mongoURI` | `MONGO_U_R_I` |

Danh sách biến của từng công cụ nằm trong README của công cụ đó.

## Dành cho người sửa code

### Cấu trúc

```text
core/               Hạ tầng dùng chung: khung tiến trình, config, logger, jsonfile, mongoclient
services/<tên>/     Mỗi service là 1 library: use case, adapter, transport, struct config
cmd/<tên>/          Mỗi deployment là 1 chương trình chạy được
go.work             Workspace để editor thấy mọi module cùng lúc
Makefile            Target chung (tidy) + tự include cmd/*/Makefile
```

Chiều phụ thuộc: `cmd` → `services` → `core`. Service không có `main` và không quyết định cách mình được chạy. Deployment dựng service và chọn cách chạy. Mỗi deployment là 1 đơn vị build và deploy, có `go.mod`, `config.yaml`, `Makefile`, `Dockerfile` riêng. Phần tiến trình (flag `-config`/`-env-file`, đọc cấu hình, logger, bắt SIGINT/SIGTERM, exit code) nằm trong `core/app` và `core/processor`, dùng chung cho mọi deployment.

### Viết service

1. `services/<tên>/`: `go mod init github.com/kienvan102/monorepo-microservices-template/services/<tên>`, `go mod edit -require=github.com/kienvan102/monorepo-microservices-template/core@v0.0.0 -replace=github.com/kienvan102/monorepo-microservices-template/core=../../core`, rồi `go work use ./services/<tên>` ở root.
2. Struct config: tag `env`. Field đường dẫn khai kiểu `config.Path`, giá trị tương đối được tính từ thư mục chứa `config.yaml`. Không khai `APP_ENV`: giá trị này có sẵn trong `app.Runtime`.
3. Transport (CLI, HTTP, worker...) implement `app.Component[C]`, với `C` là struct config: `InitFlags(fs)` khai flag, `Run(ctx, rt, cfg)` chạy. Chạy 1 lần thì `Run` return khi xong việc; chạy liên tục thì return khi `ctx` bị hủy. Transport nhận hàm dựng service từ deployment, không tự dựng.

### Viết deployment

1. `cmd/<tên>/`: `go mod init github.com/kienvan102/monorepo-microservices-template/cmd/<tên>`, `require` + `replace` tới `core` và các service dùng, rồi `go work use ./cmd/<tên>` ở root.
2. Hàm dựng service bằng google/wire (khuôn: `cmd/db-analyzer/wire.go`). Chạy `go tool wire` trong thư mục deployment, và chạy lại mỗi khi constructor của service đổi.
3. `main.go`: `processor.Main(app.New(app.Mount(transport, ...)...))` chạy mọi transport gắn vào cùng lúc; `processor.Main(app.Commands(...))` chạy 1 transport theo tên lệnh (`tool [flag chung] <lệnh> [flag của lệnh]`).
4. Deployment có nhiều service thì các service đọc chung 1 bộ cấu hình. Gắn mỗi service với 1 tiền tố riêng bằng `app.WithPrefix("x")`: service đó đọc key YAML `x:`, biến `X_...`, flag `-x-...` (flag chỉ đổi với `app.New`). Ví dụ 2 service, service thứ 2 gắn `app.WithPrefix("m2")`:

   ```yaml
   appEnv: dev            # cấp tiến trình: không bao giờ có tiền tố
   mongo:                 # service không tiền tố: MONGO_URI, MONGO_COLLECTION
     uri: mongodb://localhost:27017
     collection: threads_posts
   m2:                    # service tiền tố m2: M2_MONGO_URI, M2_MONGO_COLLECTION
     mongo:
       uri: mongodb://localhost:27017
       collection: facebook_posts_new
   ```

   Trong `.env` hay `docker run -e` thì dùng tên có tiền tố: `M2_MONGO_URI=...`. Key nào không khai dưới `m2:` thì service đó nhận giá trị mặc định trong code của nó, không lấy giá trị của service không tiền tố. Service cần deploy độc lập thì để ở deployment riêng.
5. `config.yaml`; `Makefile` với target tiền tố `<tên>-`; `Dockerfile` + `Dockerfile.dockerignore` theo khuôn của `db-analyzer`.

### Build

- Mỗi module có `go.mod` riêng. Makefile và Dockerfile build deployment với `GOWORK=off`, tức theo đúng `go.mod` của deployment, nên nâng dependency của deployment này không đổi build của deployment khác. `go.work` chỉ để editor và lệnh `go` ở root thấy mọi module.
- Deployment dùng `core` và service qua `replace` trỏ vào thư mục trong repo, nên luôn build với code hiện tại của repo.
- Root `Makefile` include mọi `cmd/*/Makefile` vào chung 1 namespace của `make`, nên target và biến mặc định của deployment phải có tiền tố theo tên deployment (`db-analyzer-collect`, `db-analyzer-%: ENV_FILE ?= ...`).
- `Dockerfile.dockerignore` theo kiểu whitelist: chỉ cho `core/`, các service mà deployment dùng và thư mục của deployment vào build context, luôn loại `.env`.
