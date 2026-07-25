# Cấu trúc source

Monorepo phát triển được chia theo quyền sở hữu và vòng đời phát hành, không theo
một workspace frontend duy nhất.

```text
.
├── backend/                 # API, worker, migration
├── frontend/                # Web chat và admin self-hosted
│   ├── apps/
│   │   ├── web/
│   │   └── admin/
│   └── packages/
├── deploy/
│   └── self-hosted/         # Installer/Compose customer
├── clients/
│   ├── mobile/              # Flutter app dùng chung
│   └── desktop/             # Tauri host dùng chung
├── portal/                  # Portal trung tâm và deploy riêng
│   ├── deploy/
│   └── download/
├── docs/
└── .github/workflows/
```

## Self-host public

`backend/`, `frontend/` và `deploy/self-hosted/` tạo thành repository public mà
customer clone. Frontend chỉ có web/admin; không chứa portal, Flutter, Tauri,
khóa ký app hay cấu hình Firebase/APNs.

Backend dùng `cmd/api`, `cmd/worker`, `internal/bootstrap`,
`internal/platform`, `internal/shared` và `internal/modules`. Frontend dùng các
workspace `apps/web`, `apps/admin` và các package typed dùng chung.

## Client chính thức

`clients/mobile` và `clients/desktop` có vòng đời release riêng. Mobile gọi
discovery của domain customer và lưu runtime trong secure storage.

Desktop là Tauri host mỏng. Để không nhân đôi UI, quá trình build đọc web UI từ
repository self-host. Trong monorepo đường dẫn được tự nhận diện; khi build ở repo
clients riêng, đặt:

```sh
WEBTUI_SELF_HOST_DIR=../webtui-chat-self-host npm run build
```

Script desktop tạo `web-dist/` cục bộ; thư mục này không được commit.

## Portal trung tâm

`portal/` là ứng dụng độc lập do VPSTTT vận hành tại
`https://chat.vpsttt.com/portal`. Portal chỉ discovery, tài liệu và download; nó
không nhận password/token và không truy cập dữ liệu chat customer.

Portal tự sở hữu `package-lock.json`, test, Dockerfile, `deploy/` và source trang
download. Vì vậy thay đổi portal không làm rebuild image self-host.

## GitHub Actions

- `ci.yml`, `docker.yml`, `deploy.yml`: backend/web/admin.
- `mobile.yml`, `desktop.yml`: official clients.
- `portal.yml`: test và image portal.

Secret ký Android, Apple, Tauri và Firebase chỉ thuộc repo clients. Secret deploy
portal chỉ thuộc repo portal. Repository self-host public không chứa các secret
này.
