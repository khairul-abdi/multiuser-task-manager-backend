# Multiuser Task Manager Backend

REST API task management multi-user sesuai requirement test: Go, PostgreSQL, GORM, JWT authentication, idempotency key, structured error handling, transactional assign task, structured JSON logging, dan unit test concurrency/idempotency tanpa koneksi database eksternal.

## Menjalankan Project Dengan Docker PostgreSQL

```bash
cp .env.example .env
# sesuaikan DATABASE_URL dan JWT_SECRET
docker compose up -d
go mod tidy
go run ./cmd/api
```

Default server berjalan di `http://localhost:8080`.

## Menjalankan Project Lokal Tanpa Docker

Pastikan PostgreSQL lokal sudah berjalan dan user database di `.env` punya akses untuk membuat/mengakses database.

```bash
cp .env.example .env
```

Sesuaikan `DATABASE_URL` di `.env`, contoh:

```env
DATABASE_URL=host=localhost user=postgres password=postgres dbname=task_manager port=5432 sslmode=disable TimeZone=Asia/Jakarta
JWT_SECRET=replace-with-long-random-secret
AUTO_MIGRATE=true
```

Buat database jika belum ada:

```bash
createdb -h localhost -p 5432 -U postgres task_manager
```

Jalankan aplikasi:

```bash
go mod tidy
go run ./cmd/api
```

Aplikasi otomatis membaca file `.env`.

## Migration

Migration memakai GORM `AutoMigrate` saat aplikasi start. Selama `AUTO_MIGRATE=true`, perintah berikut akan membuat atau memperbarui tabel `users`, `tasks`, `task_logs`, dan `idempotency_records`:

```bash
go run ./cmd/api
```

Jika ingin menjalankan aplikasi tanpa auto migration, ubah:

```env
AUTO_MIGRATE=false
```

## API Documentation

- Swagger/OpenAPI file: `docs/openapi.yaml`
- Swagger/OpenAPI served by app: `GET /swagger/openapi.yaml`
- Postman collection: `docs/postman_collection.json`

Import `docs/openapi.yaml` ke Swagger Editor/Swagger UI, atau import `docs/postman_collection.json` langsung ke Postman. Collection Postman berisi test suite lengkap yang bisa dijalankan berurutan untuk auth, CRUD task, idempotency, structured error response, authorization scope, query handling, dan assign task transaction flow.

## Environment

| Variable | Keterangan |
| --- | --- |
| `APP_PORT` | Port HTTP server |
| `DATABASE_URL` | DSN PostgreSQL untuk GORM |
| `JWT_SECRET` | Secret signing JWT |
| `JWT_TTL_HOURS` | Masa berlaku token |
| `AUTO_MIGRATE` | Jalankan GORM AutoMigrate saat startup |

## Endpoint

### Auth

- `POST /auth/register`
- `POST /auth/login`

Contoh register:

```json
{
  "name": "User One",
  "email": "user@example.com",
  "password": "password123",
  "team_id": 1
}
```

Gunakan response `token` sebagai `Authorization: Bearer <token>`.

### Tasks

- `POST /tasks` membuat task. Wajib header `Idempotency-Key` format UUID.
- `GET /tasks?status=todo&search=api&page=1&limit=10` list task dengan filter status, search title, dan pagination.
- `GET /tasks/:id` detail task.
- `PUT /tasks/:id` update task.
- `DELETE /tasks/:id` hapus task.
- `POST /tasks/:id/assign` assign task ke user lain dalam team yang sama.

Contoh create task:

```bash
curl -X POST http://localhost:8080/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: 36da41db-c714-4cfe-b18d-65f3c0e9f113" \
  -H "Content-Type: application/json" \
  -d '{"title":"Build API","description":"Task manager","status":"todo"}'
```

## Error Response

Semua error memakai format konsisten:

```json
{
  "status": 400,
  "code": "VALIDATION_ERROR",
  "message": "request body is invalid",
  "timestamp": "2026-10-12T07:00:00Z"
}
```

## Arsitektur Singkat

- `cmd/api`: entrypoint aplikasi, koneksi database, migration, router.
- `internal/domain`: model GORM dan DTO request/response.
- `internal/repository`: kontrak repository dan implementasi GORM.
- `internal/service`: business logic auth, CRUD task, idempotency, dan assign task transactional.
- `internal/handler`: HTTP handler Gin.
- `internal/middleware`: JWT auth, panic recovery, dan structured request logging.
- `internal/apperror`: error response standar.

Endpoint `POST /tasks` menyimpan response pertama berdasarkan `Idempotency-Key` selama 24 jam. Retry dengan key dan body yang sama mengembalikan response yang sama tanpa membuat task baru. Retry dengan key sama tetapi body berbeda akan ditolak dengan `409`.

Endpoint `POST /tasks/:id/assign` berjalan dalam satu database transaction: validasi actor dan assignee satu team, lock task, update assignee, insert `task_logs`, lalu mock notification via log. Jika salah satu tahap gagal, transaksi rollback.

## Testing

```bash
go test ./...
go test -race ./...
```

Unit test idempotency memakai repository in-memory, jadi tidak membutuhkan PostgreSQL atau service eksternal.
