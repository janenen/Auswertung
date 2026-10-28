# Auswertung

Evaluation of shooting results for sport shooting clubs: reads shots from a
RIKA SAG-2 scoring machine (serial port) or from CSV/QSD files and QR codes,
manages competitions, shooters, clubs, teams and RSB league matches, and
prints result reports.

Auswertung is a local web app: a single executable starts a server on
`http://127.0.0.1:8080` and opens the browser. Data is stored in the SQLite
database `./db/auswertung.db` (data of the former Python version in `./db/*.json`
is imported on first start and then moved to `./db/legacy`); nothing else is
written to disk. "Drucken / PDF" opens the browser's print dialog, where any
paper format can be chosen and the report saved as PDF.

To test without hardware, start with `-simulator`: a "Rika-Simulator" is then
offered next to the real machines and produces random shots on the
competition's target type (built-in port of https://github.com/janenen/simulator).
League competitions then also get an "Ergebnisse simulieren" button that adds
three random results each for the home and the guest team.

## Usage

Download the archive for your system from the
[releases](https://github.com/janenen/Auswertung/releases) (Windows, Linux and
macOS, each for amd64/x64 and arm64), unpack it and start `auswertung`
(`auswertung.exe` on Windows) in the folder that should contain `db`.
`SHA256SUMS` lists the checksums of the archives.

On macOS the binary is not signed: the first time, open it via right-click →
"Open", or run `xattr -d com.apple.quarantine auswertung`. Serial ports of
scoring machines appear as `/dev/cu.*` (macOS) or `/dev/ttyUSB*` (Linux; the
user needs access to them, e.g. membership in the `dialout` group).

Options:

```
auswertung -addr 127.0.0.1:8080 -data db -no-browser -simulator \
           -league-template extern/formular_ligaergebnisse_kr_bez_ll.xlsx
auswertung -version
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

Releases: pushing a version tag (e.g. `git tag 2.1.0 && git push origin 2.1.0`)
builds all platforms and publishes a GitHub release. Locally,
`scripts/build-release.sh <version>` builds the same archives into `dist/`
(after building the frontend).
