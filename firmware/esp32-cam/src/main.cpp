// Camera firmware for the AI-Thinker ESP32-CAM: joins the WiFi configured
// through a captive portal and serves an MJPEG stream for Auswertung.
//
//   port 80  /            setup page (preview, image settings, WiFi reset)
//            /status      JSON with settings and connection info
//            /capture     single JPEG
//            /control     POST ?var=<setting>&val=<value>
//            /restart     POST
//            /wifi/reset  POST, forgets the WiFi and opens the portal
//   port 81  /stream      multipart/x-mixed-replace MJPEG stream
//
// The stream runs on its own server so that the setup page and /status stay
// responsive while streaming. It serves one client at a time; Auswertung
// keeps a single connection per camera and distributes the frames.
//
// WiFi setup: without (working) saved credentials the camera opens the
// access point "auswertung-cam-xxxx" with a configuration portal. Pressing
// reset twice within two seconds opens the portal as well.

#include <Arduino.h>
#include <ESPmDNS.h>
#include <Preferences.h>
#include <WiFi.h>
#include <WiFiManager.h>

#include "camera_pins.h"
#include "esp_camera.h"
#include "esp_http_server.h"

#ifndef FIRMWARE_VERSION
#define FIRMWARE_VERSION "dev"
#endif

namespace {

constexpr uint32_t DOUBLE_RESET_WINDOW_MS = 2000;
constexpr uint32_t PORTAL_TIMEOUT_S = 300;
constexpr uint32_t CONNECT_TIMEOUT_S = 20;
constexpr uint32_t OFFLINE_RESTART_MS = 60000;
constexpr uint8_t FLASH_LEDC_CHANNEL = 7;  // camera XCLK uses channel 0

#define PART_BOUNDARY "auswertungframe"
const char *STREAM_CONTENT_TYPE = "multipart/x-mixed-replace;boundary=" PART_BOUNDARY;
// The boundary follows each frame instead of preceding it, so receivers know
// immediately that the frame is complete.
const char *STREAM_FIRST_BOUNDARY = "--" PART_BOUNDARY "\r\n";
const char *STREAM_NEXT_BOUNDARY = "\r\n--" PART_BOUNDARY "\r\n";
const char *STREAM_PART = "Content-Type: image/jpeg\r\nContent-Length: %u\r\nX-Timestamp: %ld.%06ld\r\n\r\n";

struct FrameSize {
  const char *name;
  framesize_t size;
  uint16_t width, height;
};

const FrameSize FRAME_SIZES[] = {
    {"QVGA", FRAMESIZE_QVGA, 320, 240},  {"VGA", FRAMESIZE_VGA, 640, 480},
    {"SVGA", FRAMESIZE_SVGA, 800, 600},  {"XGA", FRAMESIZE_XGA, 1024, 768},
    {"HD", FRAMESIZE_HD, 1280, 720},     {"SXGA", FRAMESIZE_SXGA, 1280, 1024},
    {"UXGA", FRAMESIZE_UXGA, 1600, 1200},
};

Preferences prefs;
WiFiManager wm;
String hostname;
httpd_handle_t controlServer = nullptr;
httpd_handle_t streamServer = nullptr;
volatile int streamClients = 0;
volatile bool restartRequested = false;
volatile bool wifiResetRequested = false;
uint8_t flashLevel = 0;

// --- settings ---

const FrameSize *frameSizeByName(const char *name) {
  for (const auto &f : FRAME_SIZES) {
    if (strcasecmp(f.name, name) == 0) return &f;
  }
  return nullptr;
}

const FrameSize *frameSizeByValue(framesize_t size) {
  for (const auto &f : FRAME_SIZES) {
    if (f.size == size) return &f;
  }
  return &FRAME_SIZES[2];
}

String defaultHostname() {
  uint8_t mac[6];
  WiFi.macAddress(mac);
  char name[32];
  snprintf(name, sizeof name, "auswertung-cam-%02x%02x", mac[4], mac[5]);
  return name;
}

// sanitizeHostname keeps a valid DNS label (a-z, 0-9, '-').
String sanitizeHostname(const char *in) {
  String out;
  for (const char *p = in; *p && out.length() < 31; p++) {
    char c = tolower(*p);
    if (isalnum(c) || c == '-') out += c;
    else if ((c == ' ' || c == '_') && out.length() && !out.endsWith("-")) out += '-';
  }
  while (out.endsWith("-")) out.remove(out.length() - 1);
  return out;
}

void setFlash(uint8_t level) {
  flashLevel = level;
  ledcWrite(FLASH_LEDC_CHANNEL, level);
}

void statusLed(bool on) { digitalWrite(STATUS_LED_GPIO, on ? LOW : HIGH); }

// --- camera ---

bool initCamera() {
  camera_config_t c = {};
  c.ledc_channel = LEDC_CHANNEL_0;
  c.ledc_timer = LEDC_TIMER_0;
  c.pin_d0 = Y2_GPIO_NUM;
  c.pin_d1 = Y3_GPIO_NUM;
  c.pin_d2 = Y4_GPIO_NUM;
  c.pin_d3 = Y5_GPIO_NUM;
  c.pin_d4 = Y6_GPIO_NUM;
  c.pin_d5 = Y7_GPIO_NUM;
  c.pin_d6 = Y8_GPIO_NUM;
  c.pin_d7 = Y9_GPIO_NUM;
  c.pin_xclk = XCLK_GPIO_NUM;
  c.pin_pclk = PCLK_GPIO_NUM;
  c.pin_vsync = VSYNC_GPIO_NUM;
  c.pin_href = HREF_GPIO_NUM;
  c.pin_sccb_sda = SIOD_GPIO_NUM;
  c.pin_sccb_scl = SIOC_GPIO_NUM;
  c.pin_pwdn = PWDN_GPIO_NUM;
  c.pin_reset = RESET_GPIO_NUM;
  c.xclk_freq_hz = 20000000;
  c.pixel_format = PIXFORMAT_JPEG;
  c.grab_mode = CAMERA_GRAB_LATEST;
  if (psramFound()) {
    // Allocate for the largest size so it can be selected later.
    c.frame_size = FRAMESIZE_UXGA;
    c.jpeg_quality = 10;
    c.fb_count = 2;
    c.fb_location = CAMERA_FB_IN_PSRAM;
  } else {
    c.frame_size = FRAMESIZE_SVGA;
    c.jpeg_quality = 12;
    c.fb_count = 1;
    c.fb_location = CAMERA_FB_IN_DRAM;
  }
  esp_err_t err = esp_camera_init(&c);
  if (err != ESP_OK) {
    Serial.printf("Kamera-Initialisierung fehlgeschlagen: 0x%x\n", err);
    return false;
  }

  sensor_t *s = esp_camera_sensor_get();
  framesize_t size = (framesize_t)prefs.getUChar("framesize", FRAMESIZE_SVGA);
  if (!psramFound() && size > FRAMESIZE_SVGA) size = FRAMESIZE_SVGA;
  s->set_framesize(s, size);
  s->set_quality(s, prefs.getUChar("quality", 12));
  s->set_vflip(s, prefs.getBool("vflip", false));
  s->set_hmirror(s, prefs.getBool("hmirror", false));
  return true;
}

// --- HTTP ---

const char SETUP_PAGE[] = R"HTML(<!doctype html>
<html lang="de"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Auswertung Kamera</title>
<style>
body{font-family:system-ui,sans-serif;margin:0;padding:1rem;background:#f4f5f7;color:#1d2330}
main{max-width:52rem;margin:auto}img{width:100%;background:#000;border-radius:6px;aspect-ratio:4/3;object-fit:contain}
fieldset{border:1px solid #ccd;border-radius:6px;margin:1rem 0;display:grid;grid-template-columns:auto 1fr;gap:.5rem 1rem;align-items:center}
code{background:#e6e8ee;padding:.1rem .3rem;border-radius:4px}button{padding:.4rem .9rem}.muted{color:#667}
</style></head><body><main>
<h1 id="title">Auswertung Kamera</h1>
<p class="muted">Stream für Auswertung: <code id="stream"></code></p>
<img id="preview" alt="Vorschau">
<fieldset><legend>Bild</legend>
<label for="framesize">Auflösung</label><select id="framesize"></select>
<label for="quality">JPEG-Qualität (4 = beste)</label><input id="quality" type="number" min="4" max="63">
<label for="vflip">Vertikal spiegeln</label><input id="vflip" type="checkbox">
<label for="hmirror">Horizontal spiegeln</label><input id="hmirror" type="checkbox">
<label for="flash">Blitz-LED</label><input id="flash" type="range" min="0" max="255">
</fieldset>
<p><button id="restart">Neu starten</button> <button id="wifi">WLAN zurücksetzen</button></p>
<p class="muted" id="info"></p>
</main><script>
const $=id=>document.getElementById(id);
const set=(v,val)=>fetch(`/control?var=${v}&val=${encodeURIComponent(val)}`,{method:'POST'});
async function load(){
  const s=await (await fetch('/status')).json();
  $('title').textContent=s.hostname;
  $('stream').textContent=s.streamUrl;
  $('framesize').innerHTML=s.framesizes.map(f=>`<option ${f==s.framesize?'selected':''}>${f}</option>`).join('');
  $('quality').value=s.quality;$('vflip').checked=s.vflip;$('hmirror').checked=s.hmirror;$('flash').value=s.flash;
  $('info').textContent=`Firmware ${s.version} · ${s.ip} · WLAN ${s.ssid} (${s.rssi} dBm) · ${s.width}×${s.height} · Stream-Clients: ${s.streamClients}`;
}
for(const id of ['framesize','quality','flash'])$(id).onchange=e=>set(id,e.target.value).then(load);
for(const id of ['vflip','hmirror'])$(id).onchange=e=>set(id,e.target.checked?1:0).then(load);
$('restart').onclick=()=>confirm('Kamera neu starten?')&&fetch('/restart',{method:'POST'});
$('wifi').onclick=()=>confirm('WLAN-Zugangsdaten löschen? Die Kamera öffnet danach wieder den Einrichtungs-Zugangspunkt.')&&fetch('/wifi/reset',{method:'POST'});
// Snapshots instead of the stream, which may be in use by Auswertung.
const preview=$('preview');
function next(){setTimeout(()=>preview.src='/capture?t='+Date.now(),300)}
preview.onload=next;preview.onerror=()=>setTimeout(next,2000);next();
load();
</script></body></html>)HTML";

esp_err_t sendJSON(httpd_req_t *req, const String &json) {
  httpd_resp_set_type(req, "application/json");
  httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  httpd_resp_set_hdr(req, "Cache-Control", "no-store");
  return httpd_resp_send(req, json.c_str(), json.length());
}

esp_err_t setupHandler(httpd_req_t *req) {
  httpd_resp_set_type(req, "text/html; charset=utf-8");
  return httpd_resp_send(req, SETUP_PAGE, sizeof SETUP_PAGE - 1);
}

esp_err_t statusHandler(httpd_req_t *req) {
  sensor_t *s = esp_camera_sensor_get();
  const FrameSize *f = frameSizeByValue(s->status.framesize);
  String ip = WiFi.localIP().toString();
  String json = "{";
  json += "\"hostname\":\"" + hostname + "\"";
  json += ",\"version\":\"" FIRMWARE_VERSION "\"";
  json += ",\"ip\":\"" + ip + "\"";
  json += ",\"ssid\":\"" + WiFi.SSID() + "\"";
  json += ",\"rssi\":" + String(WiFi.RSSI());
  json += ",\"uptime\":" + String(millis() / 1000);
  json += ",\"streamUrl\":\"http://" + ip + ":81/stream\"";
  json += ",\"streamClients\":" + String(streamClients);
  json += ",\"framesize\":\"" + String(f->name) + "\"";
  json += ",\"width\":" + String(f->width) + ",\"height\":" + String(f->height);
  json += ",\"framesizes\":[";
  for (const auto &fs : FRAME_SIZES) {
    if (!psramFound() && fs.size > FRAMESIZE_SVGA) break;
    if (fs.size != FRAME_SIZES[0].size) json += ",";
    json += "\"" + String(fs.name) + "\"";
  }
  json += "],\"quality\":" + String(s->status.quality);
  json += ",\"vflip\":" + String(s->status.vflip ? "true" : "false");
  json += ",\"hmirror\":" + String(s->status.hmirror ? "true" : "false");
  json += ",\"flash\":" + String(flashLevel);
  json += ",\"psram\":" + String(psramFound() ? "true" : "false");
  json += "}";
  return sendJSON(req, json);
}

esp_err_t captureHandler(httpd_req_t *req) {
  camera_fb_t *fb = esp_camera_fb_get();
  if (!fb) {
    httpd_resp_send_500(req);
    return ESP_FAIL;
  }
  httpd_resp_set_type(req, "image/jpeg");
  httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  httpd_resp_set_hdr(req, "Cache-Control", "no-store");
  esp_err_t res = httpd_resp_send(req, (const char *)fb->buf, fb->len);
  esp_camera_fb_return(fb);
  return res;
}

esp_err_t controlHandler(httpd_req_t *req) {
  char query[96], var[32], val[32];
  if (httpd_req_get_url_query_str(req, query, sizeof query) != ESP_OK ||
      httpd_query_key_value(query, "var", var, sizeof var) != ESP_OK ||
      httpd_query_key_value(query, "val", val, sizeof val) != ESP_OK) {
    httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "var und val fehlen");
    return ESP_FAIL;
  }
  sensor_t *s = esp_camera_sensor_get();
  int n = atoi(val);
  int res = -1;
  if (!strcmp(var, "framesize")) {
    const FrameSize *f = frameSizeByName(val);
    if (f && (psramFound() || f->size <= FRAMESIZE_SVGA) && s->set_framesize(s, f->size) == 0) {
      prefs.putUChar("framesize", f->size);
      res = 0;
    }
  } else if (!strcmp(var, "quality")) {
    if (n >= 4 && n <= 63 && (res = s->set_quality(s, n)) == 0) prefs.putUChar("quality", n);
  } else if (!strcmp(var, "vflip")) {
    if ((res = s->set_vflip(s, n != 0)) == 0) prefs.putBool("vflip", n != 0);
  } else if (!strcmp(var, "hmirror")) {
    if ((res = s->set_hmirror(s, n != 0)) == 0) prefs.putBool("hmirror", n != 0);
  } else if (!strcmp(var, "flash")) {
    setFlash(constrain(n, 0, 255));
    res = 0;
  }
  if (res != 0) {
    httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "unbekannte Einstellung oder ungültiger Wert");
    return ESP_FAIL;
  }
  return statusHandler(req);
}

esp_err_t restartHandler(httpd_req_t *req) {
  restartRequested = true;
  return sendJSON(req, "{}");
}

esp_err_t wifiResetHandler(httpd_req_t *req) {
  wifiResetRequested = true;
  return sendJSON(req, "{}");
}

esp_err_t streamHandler(httpd_req_t *req) {
  httpd_resp_set_type(req, STREAM_CONTENT_TYPE);
  httpd_resp_set_hdr(req, "Access-Control-Allow-Origin", "*");
  httpd_resp_set_hdr(req, "Cache-Control", "no-store");
  esp_err_t res = httpd_resp_send_chunk(req, STREAM_FIRST_BOUNDARY, strlen(STREAM_FIRST_BOUNDARY));
  streamClients++;
  char part[128];
  while (res == ESP_OK) {
    camera_fb_t *fb = esp_camera_fb_get();
    if (!fb) {
      Serial.println("Kein Bild von der Kamera");
      res = ESP_FAIL;
      break;
    }
    size_t n = snprintf(part, sizeof part, STREAM_PART, fb->len, (long)fb->timestamp.tv_sec, (long)fb->timestamp.tv_usec);
    res = httpd_resp_send_chunk(req, part, n);
    if (res == ESP_OK) res = httpd_resp_send_chunk(req, (const char *)fb->buf, fb->len);
    if (res == ESP_OK) res = httpd_resp_send_chunk(req, STREAM_NEXT_BOUNDARY, strlen(STREAM_NEXT_BOUNDARY));
    esp_camera_fb_return(fb);
  }
  streamClients--;
  return res;
}

void registerHandler(httpd_handle_t server, const char *uri, httpd_method_t method, esp_err_t (*handler)(httpd_req_t *)) {
  httpd_uri_t u = {};
  u.uri = uri;
  u.method = method;
  u.handler = handler;
  httpd_register_uri_handler(server, &u);
}

void startServers() {
  httpd_config_t config = HTTPD_DEFAULT_CONFIG();
  config.server_port = 80;
  config.ctrl_port = 32768;
  config.lru_purge_enable = true;
  if (httpd_start(&controlServer, &config) == ESP_OK) {
    registerHandler(controlServer, "/", HTTP_GET, setupHandler);
    registerHandler(controlServer, "/status", HTTP_GET, statusHandler);
    registerHandler(controlServer, "/capture", HTTP_GET, captureHandler);
    registerHandler(controlServer, "/control", HTTP_POST, controlHandler);
    registerHandler(controlServer, "/restart", HTTP_POST, restartHandler);
    registerHandler(controlServer, "/wifi/reset", HTTP_POST, wifiResetHandler);
  }

  config.server_port = 81;
  config.ctrl_port = 32769;
  config.max_open_sockets = 3;
  // A client that disappears without closing the connection blocks the
  // stream only until sending times out.
  config.send_wait_timeout = 5;
  if (httpd_start(&streamServer, &config) == ESP_OK) {
    registerHandler(streamServer, "/stream", HTTP_GET, streamHandler);
  }
}

// --- WiFi ---

void connectWiFi(bool forcePortal) {
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false);  // power saving makes the stream stutter

  static WiFiManagerParameter hostnameParam("hostname", "Gerätename (a-z, 0-9, -)", hostname.c_str(), 31);
  wm.addParameter(&hostnameParam);
  wm.setTitle("Auswertung Kamera");
  wm.setHostname(hostname.c_str());
  wm.setConnectTimeout(CONNECT_TIMEOUT_S);
  wm.setConfigPortalTimeout(PORTAL_TIMEOUT_S);
  wm.setAPCallback([](WiFiManager *) {
    Serial.printf("Einrichtung: mit WLAN \"%s\" verbinden, dann http://192.168.4.1 öffnen\n", hostname.c_str());
  });

  bool connected = forcePortal ? wm.startConfigPortal(hostname.c_str()) : wm.autoConnect(hostname.c_str());
  if (!connected) {
    Serial.println("Keine WLAN-Verbindung, Neustart");
    ESP.restart();
  }

  String name = sanitizeHostname(hostnameParam.getValue());
  if (name.length() && name != hostname) {
    prefs.putString("hostname", name);
    Serial.printf("Neuer Gerätename %s, Neustart\n", name.c_str());
    ESP.restart();
  }
}

}  // namespace

void setup() {
  Serial.begin(115200);
  Serial.println();
  Serial.println("Auswertung Kamera " FIRMWARE_VERSION);

  pinMode(STATUS_LED_GPIO, OUTPUT);
  ledcSetup(FLASH_LEDC_CHANNEL, 5000, 8);
  ledcAttachPin(FLASH_LED_GPIO, FLASH_LEDC_CHANNEL);
  setFlash(0);

  prefs.begin("camera");

  // Double reset: the flag survives a reset that happens while it is set.
  bool forcePortal = prefs.getBool("resetflag", false);
  prefs.putBool("resetflag", !forcePortal);
  if (!forcePortal) {
    statusLed(true);  // a reset now opens the WiFi setup
    delay(DOUBLE_RESET_WINDOW_MS);
    prefs.putBool("resetflag", false);
  }
  statusLed(false);

  WiFi.mode(WIFI_STA);  // so the MAC address can be read
  hostname = prefs.getString("hostname", defaultHostname());

  if (!initCamera()) {
    for (int i = 0; i < 20; i++) {
      statusLed(i % 2);
      delay(250);
    }
    ESP.restart();
  }

  connectWiFi(forcePortal);
  Serial.printf("Verbunden mit %s, IP %s\n", WiFi.SSID().c_str(), WiFi.localIP().toString().c_str());
  Serial.printf("Stream: http://%s:81/stream (oder http://%s.local:81/stream)\n", WiFi.localIP().toString().c_str(),
                hostname.c_str());

  if (MDNS.begin(hostname.c_str())) {
    MDNS.addService("http", "tcp", 80);
    MDNS.addServiceTxt("http", "tcp", "stream", ":81/stream");
  }
  startServers();
}

void loop() {
  static uint32_t offlineSince = 0;

  if (restartRequested) {
    delay(500);  // let the response go out
    ESP.restart();
  }
  if (wifiResetRequested) {
    delay(500);
    wm.resetSettings();
    ESP.restart();
  }

  if (WiFi.status() == WL_CONNECTED) {
    offlineSince = 0;
    statusLed(false);
  } else {
    // WiFi reconnects by itself; restart if that does not work.
    if (!offlineSince) offlineSince = millis();
    if (millis() - offlineSince > OFFLINE_RESTART_MS) ESP.restart();
    statusLed((millis() / 500) % 2);
  }
  delay(100);
}
