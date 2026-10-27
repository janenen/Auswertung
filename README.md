# Auswertung

Evaluation of shooting results for sport shooting clubs: reads shots from a
RIKA SAG-2 scoring machine (serial port) or from CSV/QSD files and QR codes,
manages competitions, shooters, clubs, teams and RSB league matches, and
creates PDF reports.

Auswertung is a local web app: a single executable starts a server on
`http://127.0.0.1:8080` and opens the browser. Data is stored in `./db`
(JSON, compatible with the former Python version) and reports are written to
`./output`.

A simulator for a RIKA machine is found in https://github.com/janenen/simulator.

## Usage

Download `auswertung.exe` from the releases and start it in the folder that
should contain `db` and `output`. Options:

```
auswertung -addr 127.0.0.1:8080 -data db -out output -no-browser \
           -league-template extern/formular_ligaergebnisse_kr_bez_ll.xlsx
```

The Excel export of league results fills the RSB form, which is not part of
this repository; place it at the `-league-template` path.

"Visualisierung" opens a fullscreen board for a second screen that cycles
through the running competitions (`#/board?interval=10`).

## Development

Requires Go ≥ 1.25 and Node.js ≥ 20.

```sh
cd web && npm install && npm run build && cd ..   # build the frontend (embedded into the binary)
go run ./cmd/auswertung                           # run the app
go test ./...                                     # backend tests
```

For frontend work, run `go run ./cmd/auswertung -no-browser` and
`npm run dev` in `web/` (Vite proxies `/api` to the Go server).
