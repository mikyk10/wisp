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
        "body{font-family:sans-serif;max-width:420px;margin:32px auto;padding:0 16px;color:#222}"
        "h1{font-size:1.4em;margin-bottom:2px}"
        "p.sub{color:#888;font-size:.85em;margin-top:0}"
        ".card{background:#f4f4f4;border-radius:8px;padding:12px 16px;margin:16px 0}"
        ".lbl{font-size:.7em;color:#999;text-transform:uppercase;letter-spacing:.05em;margin-bottom:2px}"
        ".val{font-family:monospace;font-size:1em;font-weight:bold;word-break:break-all}"
        ".val.sm{font-size:.85em;font-weight:normal;color:#555}"
        "label{display:block;margin-top:14px;font-size:.9em;color:#444}"
        "input{width:100%;box-sizing:border-box;padding:10px 8px;margin-top:4px;"
        "font-size:1em;border:1px solid #ccc;border-radius:4px}"
        "button{margin-top:20px;width:100%;padding:12px;background:#222;color:#fff;"
        "border:none;border-radius:4px;font-size:1em;cursor:pointer}"
        "</style></head><body>"
        "<h1>WiSP Setup</h1>"
        "<p class='sub'>Connect this device to your WiFi network.</p>"
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
        "<label>Password</label>"
        "<input type='password' name='password' autocomplete='new-password'";
    html += hasPassword ? " placeholder='(saved &#8212; leave blank to keep)'" : " placeholder='Password'";
    html +=
        ">"
        "<label>Server URL</label>"
        "<input type='text' name='server_url' autocomplete='off' spellcheck='false'"
        " value='";
    html += htmlEsc(savedServerURL);
    html +=
        "' placeholder='http://your-server'>"
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

    server.send(200, "text/html",
                "<!DOCTYPE html><html><head><meta charset='UTF-8'>"
                "<meta name='viewport' content='width=device-width,initial-scale=1'>"
                "<style>body{font-family:sans-serif;max-width:420px;margin:48px auto;"
                "padding:0 16px;text-align:center;color:#222}</style></head>"
                "<body><h2>Saved!</h2>"
                "<p>Connecting to WiFi&hellip;<br>You can close this page.</p>"
                "</body></html>");
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