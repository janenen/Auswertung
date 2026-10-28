# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

"Auswertung" evaluates sport-shooting results: it reads shots from scoring machines (RIKA SAG-2 via serial port) or files, manages competitions/shooters/clubs/teams/RSB league matches, and exports PDFs, CSVs and QR codes. It is a **local web app**: a Go server with an embedded Svelte 5 SPA, shipped as one executable that opens the browser. UI text is German. It replaced a Python/Tkinter app (see git history before the `go-webapp` branch); the Python code is the reference for not-yet-ported features.

For testing without hardware, `-simulator` enables a built-in RIKA simulator (`rika:SIM`, port of https://github.com/janenen/simulator) and a "Ergebnisse simulieren" button on league competitions (`POST /api/competitions/{id}/simulate`: 3 random results each for home and guest, reusing the shooters "Simulierter Heim-/Gastschütze 1–3").

## Commands

```sh
cd web && npm install && npm run build   # build SPA into web/dist (required before go build embeds it)
npm run check                            # (in web/) svelte-check type checking
npm run dev                              # (in web/) Vite dev server, proxies /api to 127.0.0.1:8080
go run ./cmd/auswertung -no-browser      # run backend (flags: -addr, -data db, -out output, -simulator)
go test ./...                            # all backend tests
go test ./internal/store -run TestLegacyImport   # single test
gofmt -l . && go vet ./...               # CI also checks these
```

CI (`.github/workflows/ci.yml`) runs frontend check/build, gofmt, vet and tests. Pushing a `*.*.*` tag builds `auswertung.exe` and publishes a GitHub release.

## Architecture

- **`internal/model`**: domain types and statistics. `Shots` carries the derived values (`Summe`, `SummeGanz`, `Best`, `AblageRL/HT`, `Series` = chunks of 10). Target geometry (`Targets`, in 1/100 mm; shot x/y are in mm) and scoring modes (`Modi`, `RankValue`, `SortMatches`; best Teiler = *lowest*) live here.
- **`internal/store`**: SQLite database `db/auswertung.db` via the pure-Go driver `modernc.org/sqlite` (no CGO, keeps the single-exe build). Normalized schema in `schema.go` with foreign keys, UUID text IDs, ISO dates (the model uses DD.MM.YYYY; the store converts), and `matches.seq` as recording/entry order; schema changes are appended to `migrations` (tracked by `PRAGMA user_version`). Every write is one transaction. `legacy.go` imports the Python version's JSON files (`db/*.json`, `../schuetzen.ini`, upgrading their old layouts in memory) once when the database is created, in the same transaction, and then moves them to `db/legacy/`; the legacy format is read-only and must not be written.
- **`internal/machine`**: `Machine` interface (`Read(ctx, Request, chan<- Reading)`). `Rika` speaks the SAG-2 serial protocol via `go.bug.st/serial` (`openPort` is swapped for a fake in tests). `CSV`, `QR`, `QSD` parse uploaded data. `simulator.go` emulates a SAG-2 as an in-memory port behind `Rika{Port: "SIM"}` (only with `SetSimulator(true)`), producing random shots on the requested target type. `Discover()` probes serial ports in parallel with a timeout; `ByID()` resolves without probing (`rika:COM3`, `csv`, `qr`, `qsd`).
- **`internal/reading`**: runs one reading per machine in a goroutine, buffers events (`status`/`shot`/`error`/`done`) and persists via a `FinishFunc` callback. Wrong target type aborts without saving; Stop(keep) saves partial results, Stop(discard) does not; on a machine error already-read shots are saved.
- **`internal/report`**: PDFs via `go-pdf/fpdf` with vector-drawn targets/arrows (`draw.go`), CSV and QR string formats that round-trip with the `machine` parsers. `Save` writes copies under `output/` with `_n` de-duplication.
- **`internal/server`**: JSON API on Go 1.22+ `ServeMux` patterns (`server.go` lists all routes), SSE at `/api/readings/{id}/events` (supports `Last-Event-ID`), report endpoints that save to `output/` and stream the file. Unknown non-API paths fall back to `index.html`.
- **`web/`**: Svelte 5 (runes) + TypeScript. `embed.go` embeds `web/dist` (`public/.gitkeep` keeps the directory non-empty for `go build`). Hash router in `src/lib/router.svelte.ts`; routes are registered in `src/App.svelte` and re-mounted on every route change (`{#key}`). `src/lib/api.ts` mirrors the server's JSON types — update both sides together. UI texts go through `t()` from `src/lib/i18n.svelte.ts` (German text = key; add every new key to `locales/en.ts`, `gu.ts`, `ml.ts`, svelte-check enforces completeness). Values from the server (scoring modes, machine names, messages) stay German and are shown via `tx()`; PDFs stay German because fpdf cannot shape Gujarati/Malayalam. `Target.svelte` renders targets as SVG in the same 1/100 mm coordinates as the backend.

Main flows: competition → entry wizard (`routes/Entry.svelte`: shooter → machine → live reading) → match view; quick analysis passes its settings in the URL query instead of creating a competition; incomplete matches (`active: true`) can be continued with the remaining shot count.

Further features:
- **League form**: `report.LeagueXLSX` fills the RSB result form with `excelize`. The template is *not* in the repo (`extern/` is gitignored); it is read from `-league-template` (default `extern/formular_ligaergebnisse_kr_bez_ll.xlsx`). Tests using it skip when it is missing.
- **Visualisation board** (`#/board`, opened in a new window for a second screen): polls `GET /api/board`, which merges saved matches with running readings (`reading.Manager.Live()`, matched by `reading.Tag`), and cycles through competitions (`?interval=` seconds).
- **Shot distribution** (`web/src/lib/Distribution.svelte`): 2D KDE contours via `d3-contour` (Scott bandwidth × 0.5 like the former seaborn plot) plus marginal box plots, rendered as SVG with resolved literal colors so "SVG speichern" exports it unchanged.
