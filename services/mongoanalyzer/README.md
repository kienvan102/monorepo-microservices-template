# mongoanalyzer

Service chứa các script phân tích mà [`db-analyzer`](../../cmd/db-analyzer/README.md) chạy. README này dành cho người viết hoặc sửa script. Muốn chạy phân tích và đọc kết quả, xem README của `db-analyzer`.

Script nằm trong `scripts/`, viết bằng JavaScript, chạy trong `mongosh`. Chúng chỉ đọc dữ liệu, không tạo/xóa index hay sửa dữ liệu.

## Script chung và script riêng

**Script chung** chỉ dùng những thông tin mà collection nào cũng có, như kích thước hay danh sách index. Tên collection được lấy từ cấu hình chứ không ghi cứng trong script. Vì vậy cùng 1 script chạy được cho mọi collection. Script chung nằm trong `scripts/common/`. Ví dụ: `common/indexes` liệt kê index của collection đang được cấu hình, collection nào cũng vậy.

**Script riêng** dùng tên field, điều kiện lọc hay query cụ thể của 1 collection, nên chỉ có nghĩa với đúng collection đó. Ví dụ, đếm đơn hàng theo field `status` chỉ có nghĩa với collection `orders`. Script riêng nằm trong `scripts/collections/<tên collection>/`, và chỉ được chạy khi collection đang cấu hình trùng tên thư mục đó.

## Thêm script

Ví dụ: thêm script riêng `status_counts` cho collection `orders`, đếm số đơn theo trạng thái.

**1. Viết script** tại `scripts/collections/orders/status_counts.js`. Tên file bắt đầu bằng chữ cái, chỉ gồm chữ, số, `_`, `-`.

```javascript
(() => {
  const options = globalThis.MONGO_ANALYSIS_OPTIONS || {};
  const orders = db.getCollection(options.collection);
  const count = (status) => orders.countDocuments({ status }, { maxTimeMS: options.queryTimeoutMS });
  const result = { pending: count('pending'), shipped: count('shipped') };
  print(`ANALYZER_RESULT_JSON:${EJSON.stringify(result)}`);
})();
```

Script được chạy bằng `mongosh`, trong một phiên đã kết nối sẵn tới database của `MONGO_DATABASE` (biến `db`). Vì vậy script viết giống hệt khi chạy tay trong `mongosh`.

Script nhận cấu hình qua biến `MONGO_ANALYSIS_OPTIONS`. `db-analyzer` tự điền biến này từ cấu hình của nó:

| Trong script | Lấy từ biến cấu hình |
| --- | --- |
| `options.collection` | `MONGO_COLLECTION` |
| `options.queryTimeoutMS` | `QUERY_TIMEOUT`, đổi ra mili giây |
| `options.sampleSize` | `SAMPLE_SIZE` |
| `options.explainLimit` | `EXPLAIN_LIMIT` |

Mọi thứ script in ra đều được lưu vào file kết quả:

- Dòng bắt đầu bằng `ANALYZER_RESULT_JSON:`, theo sau là 1 object hoặc mảng JSON: giá trị đó được ghi vào field `output_json`.
- Dòng bắt đầu bằng `ANALYZER_PROGRESS_JSON:`, theo sau là JSON: kết quả của 1 bước. Nếu script bị dừng giữa chừng, các dòng này được giữ trong field `partial_results`. Script có nhiều bước nên in dòng này sau mỗi bước.
- Phần in ra còn lại (`print`, `printjson`...) được ghi vào field `stdout`.

Cả script chạy tối đa `SCRIPT_TIMEOUT`, quá thời gian thì bị dừng. Muốn giới hạn từng query thì truyền `maxTimeMS: options.queryTimeoutMS` vào query đó. Với `explain`, đặt `maxTimeMS` ở chính lệnh `explain`. Query quá thời gian sẽ ném lỗi và làm dừng script, nên hãy bọc các bước độc lập trong `try/catch` nếu muốn các bước sau vẫn chạy tiếp.

**2. Khai script vào plan.** Plan là file JSON liệt kê các script mà `inspect`/`collect` sẽ chạy, theo đúng thứ tự trong danh sách. Có 2 loại plan:

- `scripts/plan.json`: danh sách script chung, đặt dưới key `inspect`, mỗi script ghi tên đầy đủ. `inspect` và `collect` đều chạy danh sách này.

  ```json
  { "inspect": ["common/collection_stats", "common/indexes"] }
  ```

- `scripts/collections/<tên collection>/plan.json`: danh sách script riêng của collection, đặt dưới key `default`, mỗi script chỉ ghi tên file, không có `.js` và không có đường dẫn. `collect` chạy danh sách này sau danh sách script chung.

Với ví dụ, tạo file `scripts/collections/orders/plan.json`. Nếu file đã có thì thêm `"status_counts"` vào danh sách.

```json
{ "default": ["status_counts"] }
```

**3. Chạy.** Trong cấu hình của `db-analyzer`, đặt collection là `orders` (`mongo.collection` trong `config.yaml`, hoặc `MONGO_COLLECTION` trong `.env`), rồi:

```bash
make db-analyzer-collect                                       # script chung, rồi tới status_counts
make db-analyzer-run SCRIPT=collections/orders/status_counts   # chỉ chạy status_counts
```

Kết quả ghi vào file `collections__orders__status_counts.json`. Script chưa khai vào plan vẫn chạy được bằng `make db-analyzer-run`. Các lệnh `make` tự build lại binary, nên script mới có hiệu lực ngay.
