# Ranh giới repository GitHub

## 1. `webtui-chat-self-host` (public)

Bao gồm:

- `backend/`
- `frontend/`
- `deploy/self-hosted/`
- tài liệu cài đặt, OpenAPI và kiến trúc cần cho customer
- workflow test và build image API/worker/web/admin

Không bao gồm portal, mobile, desktop, release signing, download host hoặc secret
nhà cung cấp.

## 2. `webtui-chat-clients`

Bao gồm `mobile/` và `desktop/` sau khi xuất từ `clients/`. Desktop dùng source
self-host như dependency build có version. Khi release, checkout hai repo cạnh
nhau hoặc đặt `WEBTUI_SELF_HOST_DIR`.

Repo này sở hữu Firebase/APNs configuration, Android/iOS signing và Tauri updater
signing trong GitHub Environments/Secrets. Không commit file khóa.

## 3. `webtui-chat-portal`

Nội dung của `portal/` trở thành root repository. Repo này sở hữu domain
`chat.vpsttt.com`, Docker image portal, download page và metadata release.

Portal không là bước bắt buộc trên đường truyền chat. Sau discovery, browser/app
kết nối trực tiếp tới instance customer.

## Quy tắc version

- Self-host phát hành tag `server-vX.Y.Z`.
- Client phát hành độc lập: `mobile-vX.Y.Z`, `desktop-vX.Y.Z`.
- Portal deploy theo commit SHA và tag `portal-vX.Y.Z`.
- Discovery trả `app_version` và policy minimum/recommended để client kiểm tra
  tương thích.

## Cách tách lịch sử Git

Thực hiện trên clone dự phòng sau khi commit refactor:

```sh
git filter-repo --path backend --path frontend --path deploy/self-hosted \
  --path docs --path README.md --path .env.example --path .gitignore
```

Với clients:

```sh
git filter-repo --path clients --path-rename clients/:
```

Với portal:

```sh
git filter-repo --path portal --path-rename portal/:
```

Không chạy `git filter-repo` trực tiếp trên working copy đang phát triển.
