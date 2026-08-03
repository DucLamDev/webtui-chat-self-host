# Observability, p95/p99 và cảnh báo

Stack tùy chọn gồm OpenTelemetry Collector, Tempo, Prometheus, Alertmanager và
Grafana. Profile này mặc định tắt để quickstart self-host vẫn nhẹ.

## Khởi động

Trong `deploy/self-hosted/.env`, đặt mật khẩu Grafana dài và duy nhất:

```dotenv
GRAFANA_ADMIN_PASSWORD=replace-with-a-random-password
OTEL_ENABLED=true
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
OTEL_TRACE_SAMPLE_RATIO=0.10
```

Sau đó chạy:

```bash
docker compose --profile observability up -d
```

Grafana chỉ bind loopback tại `http://127.0.0.1:3300` theo mặc định. Dùng SSH
tunnel khi quản trị từ xa; không proxy Grafana, Prometheus, Tempo hoặc
Alertmanager trực tiếp ra Internet nếu chưa có SSO/network policy.

Dashboard `WebTui / WebTui Operations` được provision tự động. Trace được API,
worker và official push relay gửi bằng OTLP/HTTP tới Collector rồi lưu trong Tempo. Khi
`OTEL_ENABLED=false`, app không tạo exporter và không phụ thuộc Collector.

## SLI có sẵn

- p95/p99 latency dựa trên histogram
  `webtui_http_request_latency_seconds_bucket`;
- tỷ lệ HTTP 5xx;
- push queue theo status, tuổi job cũ nhất, delivery rate 24 giờ và dead-letter;
- official relay queue theo `pending/processing/retry/sent/dead`, tuổi job cũ
  nhất, provider outcome 24 giờ và dead-letter;
- backup thất bại 24 giờ và thời điểm backup thành công gần nhất;
- trạng thái scrape của metric truy vấn database.

PromQL p95/p99:

```promql
histogram_quantile(0.95, sum by (le) (rate(webtui_http_request_latency_seconds_bucket{path!~"/metrics|/health|/ready"}[5m])))
histogram_quantile(0.99, sum by (le) (rate(webtui_http_request_latency_seconds_bucket{path!~"/metrics|/health|/ready"}[5m])))
```

Không dùng URL raw làm label cho route không khớp, tránh rò identifier và nổ
cardinality. Dashboard push trong `/admin` cũng không trả device token, user ID
hoặc payload.

Relay metrics được Prometheus lấy trực tiếp từ `push-relay:8090/metrics` trên
backend network. Caddy không public relay `/metrics`, `/health` hoặc `/ready`;
chỉ hai route publisher `/push-relay/v1/deliveries[/<job_id>]` được proxy. Metrics
relay chỉ là số liệu aggregate, không chứa publisher ID, token, payload hay lỗi
provider. Vì relay là profile tùy chọn, target `webtui-push-relay` có thể absent
khi profile này không chạy; rule relay không cảnh báo nếu không có series.
`WebTuiPushRelayDown` chỉ kích hoạt nếu cùng target đã từng scrape thành công
trong 24 giờ, nên instance chưa từng bật profile relay không nhận false-positive.

## Alertmanager

Rule mặc định phát hiện API down/5xx, p95/p99 cao, push queue/relay queue treo, dead-letter,
delivery rate thấp, backup chưa từng thành công sau 26 giờ, backup fail/stale và
lỗi thu metric. Alert vẫn hiện trong
Alertmanager khi chưa cấu hình receiver ngoài.

Để gửi ra email/Slack/Teams/webhook, tạo bản override cục bộ cho
`observability/alertmanager.yml`, mount file đó vào container và restart
Alertmanager. Không commit API key, SMTP password hoặc webhook URL. Kiểm tra
rule trước khi deploy:

```bash
docker compose --env-file .env -f compose.yml --profile observability run --rm \
  --no-deps --entrypoint /bin/promtool prometheus \
  check rules /etc/prometheus/rules/webtui-alerts.yml
docker compose --env-file .env -f compose.yml --profile observability run --rm \
  --no-deps --entrypoint /bin/promtool prometheus \
  check config /etc/prometheus/prometheus.yml
```

## Vận hành

- Giữ sampling production ở 0.01–0.10 trước khi đã đo dung lượng Tempo.
- Không đưa message body, token, email hoặc payload nghiệp vụ vào span attribute.
- Đặt retention phù hợp; Tempo local mặc định giữ trace 7 ngày.
- Alert `WebTuiBackupStale` chỉ có ý nghĩa sau lần backup thành công đầu tiên.
- Sau mỗi thay đổi rule, dùng `promtool check config` và gửi một alert thử tới
  receiver thật.

OpenTelemetry khuyến nghị gửi telemetry qua Collector trong production:
<https://opentelemetry.io/docs/languages/go/exporters/>. Cú pháp alert theo tài
liệu Prometheus: <https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/>.
