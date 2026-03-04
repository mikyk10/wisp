#include "WiFiManager.h"

String WiFiManager::generateHostname()
{
    uint8_t mac[6];
    WiFi.macAddress(mac);

    // MACアドレスの末尾6桁を取得
    char macSuffix[6 + 1];
    sprintf(macSuffix, "%02X%02X%02X", mac[3], mac[4], mac[5]);

    // SSIDのテンプレートを書き換え
    String hostname = hostname_template;
    hostname.replace("******", macSuffix);

    return hostname;
}

String WiFiManager::generateSSID()
{
    uint8_t mac[6];
    WiFi.macAddress(mac);

    // MACアドレスの末尾6桁を取得
    char macSuffix[6 + 1];
    sprintf(macSuffix, "%02X%02X%02X", mac[3], mac[4], mac[5]);

    // SSIDのテンプレートを書き換え
    String apSSID = ssid_template;
    apSSID.replace("******", macSuffix);
    return apSSID;
}

bool WiFiManager::connectToWiFi(const char *ssid, const char *password, int timeout)
{
    String hostname = generateHostname();
    WiFi.config(INADDR_NONE, INADDR_NONE, INADDR_NONE, INADDR_NONE);
    WiFi.setHostname(hostname.c_str());

    WiFi.begin(ssid, password);
    Serial.print("[WiFi] Connecting to ");
    Serial.println(ssid);

    int elapsed = 0;
    while (WiFi.status() != WL_CONNECTED && elapsed < timeout)
    {
        Serial.print(".");
        delay(500);
        elapsed += 500;
    }

    if (WiFi.status() == WL_CONNECTED)
    {
        Serial.println("\n[WiFi] Connected!");
        Serial.print("[WiFi] IP Address: ");
        Serial.println(WiFi.localIP());
        return true;
    }

    Serial.println("\n[WiFi] Connection failed");
    WiFi.disconnect();
    return false;
}

void WiFiManager::startSoftAP()
{
    String apSSID = generateSSID();
    WiFi.softAP(apSSID.c_str(), NULL);
    WiFi.softAPConfig(softap_ip, softap_ip, softap_subnet);

    Serial.println("[SoftAP] Activated");
    Serial.print("[SoftAP] SSID: ");
    Serial.println(apSSID);
    Serial.print("[SoftAP] IP Address: ");
    Serial.println(WiFi.softAPIP());
}

static String htmlEsc(const String &s)
{
    String out;
    out.reserve(s.length());
    for (unsigned int i = 0; i < s.length(); i++) {
        char c = s[i];
        if      (c == '&')  out += "&amp;";
        else if (c == '<')  out += "&lt;";
        else if (c == '>')  out += "&gt;";
        else if (c == '"')  out += "&quot;";
        else if (c == '\'') out += "&#39;";
        else                out += c;
    }
    return out;
}

// SVG logo body shared by both pages.
// Prepend the opening <svg> tag with desired width/height; this starts with '>' to close it.
static const char SVG_BODY[] =
    R"SVG(><defs><linearGradient id='hl_top' x1='0' y1='0' x2='1' y2='0'><stop offset='0' stop-color='#7a8faa' stop-opacity='1'/><stop offset='1' stop-color='#3a4a5e' stop-opacity='0.2'/></linearGradient><linearGradient id='hl_left' x1='0' y1='0' x2='0' y2='1'><stop offset='0' stop-color='#7a8faa' stop-opacity='1'/><stop offset='1' stop-color='#3a4a5e' stop-opacity='0.2'/></linearGradient><linearGradient id='sh_right' x1='0' y1='0' x2='0' y2='1'><stop offset='0' stop-color='#0d1018' stop-opacity='0.4'/><stop offset='1' stop-color='#050608' stop-opacity='1'/></linearGradient><linearGradient id='sh_bottom' x1='0' y1='0' x2='1' y2='0'><stop offset='0' stop-color='#0d1018' stop-opacity='0.4'/><stop offset='1' stop-color='#050608' stop-opacity='1'/></linearGradient><linearGradient id='inner_top' x1='0' y1='0' x2='0' y2='1'><stop offset='0' stop-color='#020304' stop-opacity='0.9'/><stop offset='1' stop-color='#020304' stop-opacity='0'/></linearGradient><linearGradient id='inner_left' x1='0' y1='0' x2='1' y2='0'><stop offset='0' stop-color='#020304' stop-opacity='0.9'/><stop offset='1' stop-color='#020304' stop-opacity='0'/></linearGradient><linearGradient id='mat_top' x1='0' y1='0' x2='1' y2='0'><stop offset='0' stop-color='#030406' stop-opacity='0.9'/><stop offset='1' stop-color='#030406' stop-opacity='0.2'/></linearGradient><linearGradient id='mat_left' x1='0' y1='0' x2='0' y2='1'><stop offset='0' stop-color='#030406' stop-opacity='0.9'/><stop offset='1' stop-color='#030406' stop-opacity='0.2'/></linearGradient><linearGradient id='mat_right' x1='0' y1='0' x2='0' y2='1'><stop offset='0' stop-color='#3a4e62' stop-opacity='0.2'/><stop offset='1' stop-color='#3a4e62' stop-opacity='0.7'/></linearGradient><linearGradient id='mat_bottom' x1='0' y1='0' x2='1' y2='0'><stop offset='0' stop-color='#3a4e62' stop-opacity='0.2'/><stop offset='1' stop-color='#3a4e62' stop-opacity='0.7'/></linearGradient><clipPath id='photo_clip'><rect x='25' y='25' width='70' height='70' rx='1'/></clipPath></defs><rect width='120' height='120' fill='#0d1117'/><rect x='3' y='3' width='117' height='117' rx='4' fill='#07090f'/><rect width='120' height='120' rx='4' fill='#222b3a'/><rect width='120' height='4' fill='url(#hl_top)'/><rect width='4' height='120' fill='url(#hl_left)'/><rect x='116' width='4' height='120' fill='url(#sh_right)'/><rect y='116' width='120' height='4' fill='url(#sh_bottom)'/><rect x='16' y='16' width='88' height='88' rx='2' fill='#0d1520'/><rect x='16' y='16' width='88' height='10' fill='url(#inner_top)'/><rect x='16' y='16' width='10' height='88' fill='url(#inner_left)'/><rect x='20' y='20' width='80' height='3' fill='url(#mat_top)'/><rect x='20' y='20' width='3' height='80' fill='url(#mat_left)'/><rect x='97' y='20' width='3' height='80' fill='url(#mat_right)'/><rect x='20' y='97' width='80' height='3' fill='url(#mat_bottom)'/><rect x='25' y='25' width='70' height='70' rx='1' fill='#090d14'/><g clip-path='url(#photo_clip)'><circle cx='30.1' cy='30.8' r='4.5' fill='#3ee8c0' opacity='.98'/><circle cx='88.6' cy='32.2' r='4' fill='#f5c800' opacity='.95'/><circle cx='58.9' cy='48.4' r='3.5' fill='#3ee8c0' opacity='.82'/><circle cx='41.2' cy='74' r='4.2' fill='#ff7043' opacity='.9'/><circle cx='81.2' cy='76.8' r='4.2' fill='#a855f7' opacity='.88'/><circle cx='38.6' cy='50.1' r='2.8' fill='#f03e6e' opacity='.76'/><circle cx='51.1' cy='62.9' r='2.6' fill='#ff7043' opacity='.73'/><circle cx='82.9' cy='52.7' r='2.8' fill='#f5c800' opacity='.74'/><circle cx='71.6' cy='62.3' r='2.6' fill='#a855f7' opacity='.71'/><circle cx='57.4' cy='32.2' r='1.8' fill='#f5c800' opacity='.58'/><circle cx='71.7' cy='86' r='1.6' fill='#f03e6e' opacity='.55'/></g></svg>)SVG";

void WiFiManager::handleRoot()
{
    uint8_t mac[6];
    WiFi.macAddress(mac);

    // no-colon lowercase: format used in server config (service.yaml mac_address field)
    char macConfigKey[13];
    sprintf(macConfigKey, "%02x%02x%02x%02x%02x%02x",
            mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]);

    // colon-separated: human-readable display
    char macDisplay[18];
    sprintf(macDisplay, "%02X:%02X:%02X:%02X:%02X:%02X",
            mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]);

    String hostname = generateHostname();

    // Load current saved values to pre-fill form
    String savedSSID, savedPassword, savedServerURL;
    loadCredentials(savedSSID, savedPassword);
    loadServerURL(savedServerURL);
    bool hasPassword = savedPassword.length() > 0;

    String html =
        "<!DOCTYPE html><html lang='en'>"
        "<head><meta charset='UTF-8'>"
        "<meta name='viewport' content='width=device-width,initial-scale=1'>"
        "<title>WiSP Setup</title>"
        "<style>"
        "body{font-family:sans-serif;max-width:420px;margin:32px auto;padding:0 16px;"
        "background:#0f1117;color:#e0e0e0}"
        "h1{font-size:1.4em;margin-bottom:2px}"
        "p.sub{color:#888;font-size:.85em;margin-top:0}"
        ".card{background:#1a1d27;border:1px solid #252836;border-radius:8px;padding:12px 16px;margin:16px 0}"
        ".lbl{font-size:.7em;color:#6e7681;text-transform:uppercase;letter-spacing:.05em;margin-bottom:2px}"
        ".val{font-family:monospace;font-size:1em;font-weight:bold;word-break:break-all;color:#00d2a8}"
        ".val.sm{font-size:.85em;font-weight:normal;color:#888}"
        "label{display:block;margin-top:14px;font-size:.9em;color:#ccc}"
        "input{width:100%;box-sizing:border-box;padding:10px 8px;margin-top:4px;"
        "font-size:1em;border:1px solid #252836;border-radius:4px;"
        "background:#1a1d27;color:#e0e0e0}"
        "button{margin-top:20px;width:100%;padding:12px;background:#00d2a8;color:#0f1117;"
        "border:none;border-radius:4px;font-size:1em;font-weight:bold;cursor:pointer}"
        "</style></head><body>"
        "<div style='display:flex;align-items:center;gap:12px;margin-bottom:4px'>"
        "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 120 120' width='48' height='48'";
    html += SVG_BODY;
    html +=
        "<div>"
        "<h1 style='margin:0'>WiSP Setup</h1>"
        "<p class='sub' style='margin:2px 0 0'>Connect this device to your WiFi network.</p>"
        "</div>"
        "</div>"
        "<div class='card'>"
        "<div class='lbl'>Device ID &mdash; use this in server config</div>"
        "<div class='val'>";
    html += macConfigKey;
    html +=
        "</div>"
        "<div style='margin-top:8px'>"
        "<span class='lbl'>MAC &nbsp;</span><span class='val sm'>";
    html += macDisplay;
    html +=
        "</span>&nbsp;&nbsp;"
        "<span class='lbl'>Hostname &nbsp;</span><span class='val sm'>";
    html += hostname;
    html +=
        "</span></div>"
        "</div>"
        "<form action='/save' method='POST' autocomplete='off'>"
        "<datalist id='ssids'></datalist>"
        "<label>WiFi SSID</label>"
        "<input type='text' name='ssid' list='ssids' autocomplete='off' spellcheck='false'"
        " value='";
    html += htmlEsc(savedSSID);
    html +=
        "' placeholder='e.g. MyHomeNetwork'>"
        "<label>WiFi Password</label>"
        "<input type='password' name='password' autocomplete='new-password'";
    html += hasPassword ? " placeholder='(saved &#8212; leave blank to keep)'" : " placeholder='Password'";
    html +=
        ">"
        "<label>Server URL</label>"
        "<input type='text' name='server_url' autocomplete='off' spellcheck='false'"
        " value='";
    html += htmlEsc(savedServerURL);
    html +=
        "' placeholder='http://192.168.x.x:9002'>"
        "<small style='color:#6e7681;font-size:.75em'>"
        "Enter WiSP Server address and port (e.g. http://192.168.1.10:9002)"
        "</small>"
        "<button type='submit'>Save &amp; Connect</button>"
        "</form>"
        "<script>"
        "fetch('/scan').then(function(r){return r.json();}).then(function(list){"
        "var dl=document.getElementById('ssids');"
        "list.forEach(function(s){var o=document.createElement('option');o.value=s;dl.appendChild(o);});"
        "});"
        "</script>"
        "</body></html>";

    server.send(200, "text/html", html);
}

void WiFiManager::handleScan()
{
    int n = WiFi.scanNetworks();
    String json = "[";
    for (int i = 0; i < n; i++)
    {
        if (i > 0) json += ",";
        String ssid = WiFi.SSID(i);
        ssid.replace("\\", "\\\\");
        ssid.replace("\"", "\\\"");
        json += "\"";
        json += ssid;
        json += "\"";
    }
    json += "]";
    WiFi.scanDelete();
    server.send(200, "application/json", json);
}

void WiFiManager::handleSave()
{
    if (!server.hasArg("ssid") || !server.hasArg("server_url"))
    {
        server.send(400, "text/plain", "Missing parameters");
        return;
    }

    String newSSID      = server.arg("ssid");
    String newPassword  = server.arg("password");
    String newServerURL = server.arg("server_url");

    // Empty password → keep existing
    if (newPassword.length() == 0)
    {
        String existingSSID, existingPassword;
        loadCredentials(existingSSID, existingPassword);
        newPassword = existingPassword;
    }

    Serial.println("[WiFi] Saving new settings...");
    saveCredentials(newSSID.c_str(), newPassword.c_str());
    saveServerURL(newServerURL.c_str());

    String saveHtml =
        "<!DOCTYPE html><html><head><meta charset='UTF-8'>"
        "<meta name='viewport' content='width=device-width,initial-scale=1'>"
        "<style>body{font-family:sans-serif;max-width:420px;margin:48px auto;"
        "padding:0 16px;text-align:center;background:#0f1117;color:#e0e0e0}</style></head>"
        "<body><div style='text-align:center'>"
        "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 120 120' width='64' height='64'"
        " style='display:block;margin:0 auto 12px'";
    saveHtml += SVG_BODY;
    saveHtml +=
        "<h2>Saved!</h2>"
        "<p>Connecting to WiFi&hellip;<br>You can close this page.</p>"
        "</div></body></html>";
    server.send(200, "text/html", saveHtml);
    delay(2000);
    ESP.restart();
}

void WiFiManager::startSoftAPWithWebServer()
{
    startSoftAP();

    enableMDNS();

    server.on("/", HTTP_GET, std::bind(&WiFiManager::handleRoot, this));
    server.on("/scan", HTTP_GET, std::bind(&WiFiManager::handleScan, this));
    server.on("/save", HTTP_POST, std::bind(&WiFiManager::handleSave, this));

    server.begin();
    Serial.println("[SoftAP] Web server started");

    while (true) {
        server.handleClient();
        delay(10);
    }
}

bool WiFiManager::loadCredentials(String &ssid, String &password)
{
    preferences.begin("wifi", true);
    ssid = preferences.getString("ssid", "");
    password = preferences.getString("password", "");
    preferences.end();

    Serial.println("[WiFi] Validating saved credentials");
    if (ssid.length() > 0 && password.length() > 0)
    {
        Serial.println("[WiFi] Loaded saved credentials");
        return true;
    }

    Serial.println("[WiFi] No saved WiFi credentials found");
    return false;
}

void WiFiManager::saveCredentials(const char *ssid, const char *password)
{
    preferences.begin("wifi", false);
    preferences.putString("ssid", ssid);
    preferences.putString("password", password);
    preferences.end(); // 保存後に閉じる
    Serial.println("[WiFi] Credentials saved to EEPROM");
}

void WiFiManager::saveServerURL(const char *url)
{
    preferences.begin("wifi", false);
    preferences.putString("server_url", url);
    preferences.end();
    Serial.println("[WiFi] Server URL saved");
}

bool WiFiManager::loadServerURL(String &url)
{
    preferences.begin("wifi", true);
    url = preferences.getString("server_url", "");
    preferences.end();
    return url.length() > 0;
}

void WiFiManager::enableMDNS()
{
    if (!MDNS.begin("wisp"))
    {
        Serial.println("[mDNS] Error starting mDNS");
        return;
    }
    Serial.println("[mDNS] Service started at http://wisp.local/");
    MDNS.addService("http", "tcp", 80);
}