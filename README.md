# Go REST API Template

Reusable backend scaffold untuk REST API menggunakan **Gin**, **Swagger (Swag + gin-swagger)**, dan **GORM** (Postgres), dengan autentikasi **JWT + role**, serta contoh resource CRUD yang terproteksi (Todo).

## Fitur

- Autentikasi: register, login (bcrypt + JWT HS256)
- Role-based access: `user` dan `admin`
- CRUD Todo dengan ownership check (hanya pemilik atau admin)
- Dokumentasi API Swagger otomatis di `/swagger/index.html`
- Struktur berlapis: `handlers → services → models`, plus `middleware`, `dto`, `config`
- Koneksi PostgreSQL (kompatibel dengan Neon) via GORM `AutoMigrate`
- Test berjalan di SQLite in-memory (tanpa database nyata)

## Persyaratan (Prerequisites)

| Tool | Minimal | Keterangan |
|------|---------|------------|
| Go | 1.26 | Versi saat ini di `go.mod` |
| PostgreSQL | 14+ | Bisa lokal atau Neon (connection string) |
| swag CLI | latest | Hanya dibutuhkan jika ingin **regenerate** dokumentasi Swagger |

Install swag CLI (opsional, hanya untuk regenerate docs):

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

## Konfigurasi

Salin `.env.example` menjadi `.env`, lalu isi nilainya:

```bash
cp .env.example .env
```

| Key | Wajib | Deskripsi |
|-----|-------|-----------|
| `POSTGRESQL_DATABASE` | ✅ | DSN Postgres, contoh: `postgresql://user:pass@host:5432/dbname?sslmode=require` |
| `JWT_SECRET` | ✅ | Secret untuk menandatangani token. Gunakan string panjang acak (mis. `openssl rand -hex 32`) |
| `PORT` | — | Port server (default `8080`) |
| `TOKEN_HOURS` | — | Umur token dalam jam (default `24`) |
| `GIN_MODE` | — | `debug` / `release` (default `debug`) |

> `.env` sudah di-`gitignore` — jangan commit ke repositori.

## Menjalankan

```bash
go run .
```

Server berjalan di `http://localhost:8080` (atau port sesuai `PORT`).

### Regenerate dokumentasi Swagger

Setiap kali mengubah anotasi `@` pada handler:

```bash
swag init
```

Output: `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go`.

## Menjalankan Test

```bash
go test ./...          # semua test
go test ./... -race    # dengan race detector
```

Test memakai SQLite in-memory (`github.com/glebarez/sqlite`) — tidak butuh database.

## Endpoint

Buka dokumentasi interaktif: **http://localhost:8080/swagger/index.html**

| Method | Path | Auth | Deskripsi |
|--------|------|------|-----------|
| `GET` | `/healthz` | — | Health check |
| `POST` | `/api/v1/auth/register` | — | Daftar akun baru (role `user`) |
| `POST` | `/api/v1/auth/login` | — | Login, mengembalikan JWT |
| `GET` | `/api/v1/users/me` | ✅ | Profil user saat ini |
| `GET` | `/api/v1/users` | ✅ admin | Daftar semua user |
| `GET` | `/api/v1/todos` | ✅ | Daftar todo milik sendiri |
| `POST` | `/api/v1/todos` | ✅ | Buat todo |
| `GET` | `/api/v1/todos/:id` | ✅ | Ambil satu todo (pemilik/admin) |
| `PUT` | `/api/v1/todos/:id` | ✅ | Ubah todo (pemilik/admin) |
| `DELETE` | `/api/v1/todos/:id` | ✅ | Hapus todo (pemilik/admin) |

### Contoh penggunaan

Register:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"password123"}'
```

Login:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"password123"}'
# => {"token":"<JWT>"}
```

Akses route terproteksi:

```bash
curl http://localhost:8080/api/v1/todos \
  -H 'Authorization: Bearer <JWT>'
```

## Struktur Proyek

```
go-backend/
├── main.go                  # wiring: config, DB, migrate, router, swagger
├── docs/                    # generated Swagger (swag init)
├── pkg/
│   ├── config/              # baca env → Config
│   ├── database/            # koneksi GORM + AutoMigrate
│   ├── middleware/          # CORS, RequireAuth, RequireRole, CurrentUser
│   └── token/               # generate & parse JWT
└── internal/
    ├── models/              # User, Todo
    ├── dto/                 # request/response structs
    ├── services/            # logic + akses DB
    ├── handlers/            # HTTP layer + anotasi swag
    └── router/              # registrasi route & middleware
```

## Membuat user admin

`/api/v1/auth/register` selalu membuat role `user`. Untuk akun `admin`, set langsung di database:

```sql
UPDATE users SET role = 'admin' WHERE email = 'demo@example.com';
```
