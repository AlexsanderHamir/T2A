# T2A

**T2A** is for **delegating lots of tasks to agents** and keeping humans and automation on the same page. Tasks live in **one shared place**, you **create and update them through a web API**, every important change is **recorded** (who did what), and **UIs or runners can listen for updates** instead of polling constantly.

This repo is the Go implementation (**`github.com/AlexsanderHamir/T2A`**). For a fuller picture of how it works and where the rough edges are, see **`docs/DESIGN.md`**.

## Prerequisites

- **Go** 1.25+
- A **Postgres** database and a **`.env`** file at the repo root (gitignored) with **`DATABASE_URL`** pointing at it.

## Build and test

```bash
go build ./...
go test ./...
```

## Run

```bash
go run ./cmd/dbcheck    # checks the database; add -migrate to update tables
go run ./cmd/taskapi    # starts the web server; add -h for -port, -env, -migrate
```

With **`taskapi`** running (by default **`http://127.0.0.1:8080`**):

- Work with tasks at **`/tasks`** and **`/tasks/{id}`** — methods, query options, and error behavior are described in **`docs/DESIGN.md`**.
- Open **`/events`** in a client that supports **live streams** to hear when tasks change (same doc explains the format).

**Windows PowerShell:** use **`curl.exe`** and single-quoted JSON so Windows does not treat `curl` as a different command:

```powershell
curl.exe -s -X POST http://127.0.0.1:8080/tasks -H "Content-Type: application/json" -d '{"title":"live"}'
curl.exe -N http://127.0.0.1:8080/events
```

## For developers

Route and type details live next to the code. From the repo root:

```bash
go doc -all ./pkgs/tasks/...
go doc -all ./internal/envload ./cmd/taskapi ./cmd/dbcheck
```
