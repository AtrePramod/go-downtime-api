# go-downtime-api

A small REST API for tracking **machine downtime on a factory floor**. Operators log when a machine stops, why it stopped (breakdown, no material, changeover and so on) and for how long. A daily report then shows where the time went: total minutes lost, the top reasons, the worst machine, and downtime per production line.

I built it to learn Go coming from Node.js/NestJS. The daily report has **two implementations**, one running its database queries one after another and one running them in parallel with goroutines, so the two can be compared.

## Tech stack

| | |
|---|---|
| Language | Go 1.27 |
| Web framework | [Echo v4](https://echo.labstack.com/) |
| ORM | [GORM](https://gorm.io/) with the PostgreSQL driver (pgx) |
| Database | PostgreSQL (tested on 17) |
| Concurrency | goroutines + [`errgroup`](https://pkg.go.dev/golang.org/x/sync/errgroup) |
| Config | `.env` file loaded with [godotenv](https://github.com/joho/godotenv) |
| Tests | Go's built-in `testing` package and `httptest` |

## Project structure

```
.
├── main.go                 # starts Echo, registers routes
├── config/db.go            # DB connection, pool settings, AutoMigrate
├── models/                 # GORM models: Machine, DowntimeLog (+ tests)
├── handlers/               # HTTP handlers, validation, daily report (+ tests)
├── seed/seed.go            # fills the DB with sample data: go run ./seed
├── scripts/
│   ├── curl-examples.sh    # curl for every endpoint, incl. error cases
│   └── compare-report.ps1  # benchmarks sequential vs concurrent report
├── postman/                # Postman collection with tests
└── Dockerfile
```

## Getting started

### 1. Prerequisites

- [Go](https://go.dev/dl/) 1.27 or newer
- PostgreSQL running locally (or anywhere you can reach)

### 2. Create the database

In `psql` or pgAdmin's Query Tool:

```sql
CREATE DATABASE downtime_db;
```

The tables are created automatically when the app starts.

### 3. Configure environment variables

Copy the example file and put in your own PostgreSQL password:

```bash
cp .env.example .env        # Windows PowerShell: Copy-Item .env.example .env
```

| Variable | Example | Description |
|---|---|---|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | `your_postgres_password` | That user's password |
| `DB_NAME` | `downtime_db` | Database name |
| `PORT` | `8080` | HTTP port for the API (optional) |

`.env` is listed in `.gitignore` and is never committed.

### 4. Seed sample data (optional)

```bash
go run ./seed
```

This creates **5 machines across 2 lines** and **200 random downtime logs** spread over the last 7 days. It empties the `machines` and `downtime_logs` tables first, so you can run it again whenever you want a fresh data set.

### 5. Run the API

```bash
go run .
```

```
connected to PostgreSQL
database migrated
⇨ http server started on [::]:8080
```

Check that it's up: `curl localhost:8080/health` should return `{"status":"ok"}`.

### 6. Run the tests

```bash
go test ./...        # all tests
go test ./... -v     # list every test case
```

The tests cover the minutes calculation, input validation, and HTTP error responses. They don't need a database.

### Run with Docker (optional)

```bash
docker build -t go-downtime-api .

# The API (connects to PostgreSQL on your host machine)
docker run --rm -p 8080:8080 --env-file .env \
  -e DB_HOST=host.docker.internal -e TZ=Asia/Kolkata \
  go-downtime-api

# Seed the database from the same image
docker run --rm --env-file .env -e DB_HOST=host.docker.internal go-downtime-api /app/seed
```

Set `TZ` to your own time zone. The `?date=` filters use the server's local day, and a container defaults to UTC.

## API

All request and response bodies are JSON. **Every error uses the same shape:**

```json
{ "error": "message" }
```

| Status | When |
|---|---|
| `200 OK` | Successful read |
| `201 Created` | Resource created |
| `400 Bad Request` | Invalid JSON, missing or invalid fields, bad query parameters |
| `404 Not Found` | Unknown `machine_id`, or unknown route |
| `409 Conflict` | A machine with that name already exists |
| `500 Internal Server Error` | Unexpected server or database error (details are logged, never returned) |

Times use **RFC 3339** format, e.g. `2026-11-04T08:30:00+05:30` or `2026-11-04T03:00:00Z`.

### Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/machines` | Create a machine |
| `GET` | `/machines` | List all machines |
| `POST` | `/downtime` | Log a downtime period |
| `GET` | `/downtime` | List downtime logs (filters: `machine_id`, `date`) |
| `GET` | `/reports/daily` | Daily downtime report |

---

### `POST /machines`

`name` and `line` are required. Names must be unique.

```bash
curl -X POST localhost:8080/machines \
  -H 'Content-Type: application/json' \
  -d '{"name":"Lathe-01","line":"Line B"}'
```

`201 Created`
```json
{
  "id": 6,
  "name": "Lathe-01",
  "line": "Line B",
  "created_at": "2026-11-04T21:30:00.81+05:30"
}
```

Errors: `400` `{"error":"name is required"}` · `400` `{"error":"line is required"}` · `409` `{"error":"a machine with this name already exists"}`

---

### `GET /machines`

```bash
curl localhost:8080/machines
```

`200 OK`
```json
[
  { "id": 1, "name": "CNC-01", "line": "Line A", "created_at": "2026-11-04T21:25:22.40+05:30" },
  { "id": 4, "name": "Packer-01", "line": "Line B", "created_at": "2026-11-04T21:25:22.40+05:30" }
]
```

---

### `POST /downtime`

`reason` must be one of: `Breakdown`, `No material`, `Changeover`, `Quality issue`, `Power cut`.
The server works out `minutes` from the two times and drops any leftover seconds. Clients don't send it.

```bash
curl -X POST localhost:8080/downtime \
  -H 'Content-Type: application/json' \
  -d '{
    "machine_id": 1,
    "reason": "Breakdown",
    "start_time": "2026-11-04T08:00:00+05:30",
    "end_time": "2026-11-04T08:45:30+05:30"
  }'
```

`201 Created`
```json
{
  "id": 201,
  "machine_id": 1,
  "reason": "Breakdown",
  "start_time": "2026-11-04T08:00:00+05:30",
  "end_time": "2026-11-04T08:45:30+05:30",
  "minutes": 45,
  "created_at": "2026-11-04T21:30:00.98+05:30"
}
```

| Problem | Response |
|---|---|
| `machine_id` doesn't exist | `404` `{"error":"machine not found"}` |
| `end_time` not after `start_time` | `400` `{"error":"end_time must be after start_time"}` |
| Unknown reason | `400` `{"error":"reason must be one of: Breakdown, No material, Changeover, Quality issue, Power cut"}` |
| Missing fields | `400` `{"error":"machine_id is required"}` / `{"error":"start_time and end_time are required"}` |
| Time not in RFC 3339 format, or broken JSON | `400` `{"error":"invalid JSON body (times must be RFC 3339, e.g. 2026-11-04T08:30:00Z)"}` |

---

### `GET /downtime`

Newest first. Both filters are optional and can be combined:

- `machine_id`: only logs for that machine
- `date` (`YYYY-MM-DD`): only logs that **started** on that day

```bash
curl "localhost:8080/downtime?machine_id=1&date=2026-11-03"
```

`200 OK`
```json
[
  {
    "id": 96,
    "machine_id": 1,
    "reason": "No material",
    "start_time": "2026-11-03T15:09:00+05:30",
    "end_time": "2026-11-03T17:20:00+05:30",
    "minutes": 131,
    "created_at": "2026-11-04T21:25:22.40+05:30"
  }
]
```

Errors: `400` `{"error":"date must be in YYYY-MM-DD format"}` · `400` `{"error":"machine_id must be a positive integer"}`

---

### `GET /reports/daily`

| Query parameter | Required | Description |
|---|---|---|
| `date` | yes | Day to report on, `YYYY-MM-DD` |
| `mode` | no | `concurrent` (default) or `sequential` |

```bash
curl "localhost:8080/reports/daily?date=2026-11-03"
curl "localhost:8080/reports/daily?date=2026-11-03&mode=sequential"
```

`200 OK`
```json
{
  "date": "2026-11-03",
  "mode": "concurrent",
  "total_downtime_minutes": 1843,
  "top_reasons": [
    { "reason": "Power cut", "minutes": 615 },
    { "reason": "No material", "minutes": 431 },
    { "reason": "Breakdown", "minutes": 314 }
  ],
  "worst_machine": { "id": 5, "name": "Filler-01", "line": "Line B", "minutes": 620 },
  "downtime_by_line": [
    { "line": "Line B", "minutes": 942 },
    { "line": "Line A", "minutes": 901 }
  ],
  "took_ms": 0.51
}
```

- `top_reasons`: up to 3 reasons, highest minutes first.
- `worst_machine`: `null` if there was no downtime that day.
- `took_ms`: how long the four report queries took on the server, in milliseconds.

Errors: `400` `{"error":"date is required (YYYY-MM-DD)"}` · `400` `{"error":"date must be in YYYY-MM-DD format"}` · `400` `{"error":"mode must be 'concurrent' or 'sequential'"}`

---

### Testing the endpoints

- **curl:** `bash scripts/curl-examples.sh` (Git Bash on Windows) runs every endpoint, including the error cases. Set `BASE=http://localhost:8081` to use another port.
- **Postman:** import `postman/go-downtime-api.postman_collection.json` and use **Run collection**. Every request has tests for its status code and error shape.

## Sequential vs concurrent report

The daily report needs four independent queries: total minutes, top reasons, worst machine, and minutes per line. Both versions are in [`handlers/reports.go`](handlers/reports.go).

**`reportSequential`** runs them one after another:

```
total ─────▶ top reasons ─────▶ worst machine ─────▶ by line
time ≈ q1 + q2 + q3 + q4
```

**`reportConcurrent`** (the default) starts each query in its own goroutine using `errgroup`:

```
total          ─────▶
top reasons    ─────▶
worst machine  ─────▶      g.Wait()
by line        ─────▶
time ≈ the slowest query
```

```go
g, gctx := errgroup.WithContext(ctx)
g.Go(func() error { var err error; total, err = h.queryTotalMinutes(gctx, from, to); return err })
g.Go(func() error { var err error; top, err = h.queryTopReasons(gctx, from, to); return err })
// ...
if err := g.Wait(); err != nil { return nil, err } // first error wins; the rest are cancelled
```

**How it avoids race conditions:**

1. Each goroutine writes to **its own variable**. No two goroutines share one, so no mutex is needed.
2. Results are read only **after `g.Wait()`**, which guarantees every goroutine has finished.
3. `*gorm.DB` is safe to share: each query starts a new chain, and the connection pool gives each one its own connection.
4. If one query fails, `errgroup` cancels the shared context so the others stop early, and `Wait` returns that first error.

**Connection pool:** Go's `database/sql` keeps only 2 idle connections by default. With 4 parallel queries, every request had to open 2 new connections, which made the concurrent version **slower** (about 35 ms vs about 1 ms). Setting `SetMaxIdleConns(10)` in [`config/db.go`](config/db.go) fixed it.

**Comparing them:**

```powershell
.\scripts\compare-report.ps1 -Date 2026-11-03 -Runs 20
```

On a local database with 200 rows both take under 1 ms, so the difference is small. Each query is so quick that the fixed cost of starting goroutines and taking connections from the pool is a large share of the time. (On Windows the timer also moves in steps of about 0.5 ms, so look at medians over many runs.)

The concurrent version pulls ahead when:

- **queries are slow** (large tables, heavy joins, missing indexes),
- there is **network latency** (a cloud database 20 ms away costs 4 × 20 ms sequentially but about 20 ms concurrently),
- the report needs **more queries**.

To check for data races, run the app with Go's race detector, which needs CGO and a C compiler such as GCC:

```bash
CGO_ENABLED=1 go run -race .
```

## What I learned

1. _TODO_
2. _TODO_
3. _TODO_
