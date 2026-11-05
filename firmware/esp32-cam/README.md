# ESP32-CAM firmware

Firmware for an AI-Thinker ESP32-CAM (OV2640, 4 MB PSRAM). It films a target
and sends an MJPEG stream to Auswertung, which shows all cameras tiled in the
camera window (`#/cameras`). A planned camera calibration will later turn the
image into an orthonormal (top-down) view of the target.

## Build and flash

You need [PlatformIO](https://platformio.org/) (CLI or the VS Code extension).

```sh
cd firmware/esp32-cam
pio run                        # build
pio run -t upload              # flash (USB-serial adapter or ESP32-CAM-MB board)
pio device monitor             # serial log, 115200 baud
```

To flash without the ESP32-CAM-MB board, connect GPIO0 to GND, then reset.
Remove the bridge and reset again when flashing is done.

## WiFi setup

1. If the camera has no WiFi credentials, or cannot connect for 20 s, it opens
   the access point **`auswertung-cam-xxxx`** (the last four hex digits of its
   MAC address).
2. Connect a phone or laptop to it. The configuration portal opens on its own,
   or go to <http://192.168.4.1>.
3. Choose the WiFi network the Auswertung computer is on and enter its
   password. You can also change the device name, which is the hostname and
   mDNS name.
4. The camera joins the network. The serial log shows its IP address, and so
   does the router's client list.

If nobody configures it within 5 minutes, the camera restarts and tries the
saved network again. To open the portal later, either **press reset twice
within 2 seconds** (the red LED on the back lights up during that window), or
click "WLAN zurücksetzen" on the setup page.

## Add the camera to Auswertung

Go to Stammdaten → Kameras → "+ Neu", give the camera a name (e.g. "Stand 1")
and enter its IP address. A bare address resolves to
`http://<ip>:81/stream`. Other MJPEG cameras work too if you enter their full
stream URL. Use the IP address rather than `<name>.local`: the release builds
of Auswertung cannot resolve mDNS names on Linux.

To keep the address from changing, set up a DHCP reservation in the router.

## HTTP interface

| Port | Path          | Method | Description                                              |
|------|---------------|--------|----------------------------------------------------------|
| 80   | `/`           | GET    | Setup page: preview, resolution, quality, mirroring, flash LED |
| 80   | `/status`     | GET    | JSON: name, IP, RSSI, resolution, settings                |
| 80   | `/capture`    | GET    | Single JPEG                                               |
| 80   | `/control`    | POST   | `?var=framesize&val=SVGA` (QVGA…UXGA), `quality` (4–63), `vflip`/`hmirror` (0/1), `flash` (0–255) |
| 80   | `/restart`    | POST   | Restart                                                   |
| 80   | `/wifi/reset` | POST   | Erase the WiFi credentials and restart into the portal    |
| 81   | `/stream`     | GET    | `multipart/x-mixed-replace` MJPEG stream                  |

The camera saves its settings in flash. The stream sends the boundary after
each frame, so a receiver can process a frame as soon as it arrives. Each
part has an `X-Timestamp` header with the capture time in seconds since boot.

The stream server handles **one client at a time**. Auswertung opens a single
connection per camera and passes the frames on to every viewer
(`internal/camera`). While Auswertung is streaming, the setup page shows
snapshots (`/capture`), not the stream.
