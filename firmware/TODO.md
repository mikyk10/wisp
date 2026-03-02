# Firmware TODO

## 未対応の既知問題

### ~~busyHigh() タイムアウトなし~~ 対応済み

60秒タイムアウトを追加。超過時は `esp_deep_sleep_start()` で1時間スリープ後にリトライ。
全3ドライバ（epd7in3e / epd4ine6 / epd13in3e）に適用済み。

---

### ~~sendImageData() ストリームハング~~ 対応済み

30秒無受信タイムアウトを追加。`lastRecv` をデータ受信のたびにリセットし、
超過時は `sleepOnError()` で1時間スリープ後にリトライ。
全3ドライバに適用済み（epd13in3e は CS_M / CS_S 各ループにそれぞれ適用）。

---

### epd4ine6 enterSleep() — POWER_OFF コマンド抜け（要調査）

**対象**: `epd4ine6/EPaperDisplayImpl.cpp:192`

`epd7in3e` は `enterSleep()` 冒頭で `POWER_OFF (0x02)` を送ってから Deep Sleep (0x07) を送るが、
`epd4ine6` は POWER_OFF なしでいきなり 0x07 を送っている。

デバイスのサンプルプログラムからの移植のため意図が不明。
Waveshare の `epd4in0e` 系データシートと公式サンプルコードで正しいシーケンスを確認すること。

---

### epd4ine6 の解像度定義が疑わしい（要調査）

**対象**: `epd4ine6/EPaperDisplayImpl.cpp`

```cpp
#define EPD_WIDTH  600
#define EPD_HEIGHT 400
```

`initialize()` 内の解像度設定コマンド (0x61) は以下を送信している：

```
0x01 0x90 → 0x0190 = 400
0x02 0x58 → 0x0258 = 600
```

コマンドの引数が幅・高さどちらの順かはデータシート次第だが、`epd7in3e` のコードをそのままコピーしている可能性が高い。
Waveshare の 4.0inch E-Paper E のデータシートで実際の解像度とコマンド仕様を確認すること。
送るデータ量がパネルの実際の画素数と一致しない場合、表示が崩れるか最終行が欠ける。

---

### epd7in3e / epd4ine6 コードの重複（リファクタリング候補）

**対象**: 両実装全体

`sendImageData`, `sendCommand`, `sendData`, `busyHigh`, `reset`, `moduleInit` の実装が
2ファイルでほぼ同一。ハードウェア差分（初期化コマンド列）以外の共通ロジックを
基底クラス `WaveshareEPaperBase` 等に切り出すことで保守性が上がる。優先度は低い。

---

### EPD13IN3E — 実機テスト前の準備

ドライバ実装は完了済み。実機入手後に以下を対応すること。

**1. `platformio.ini` に環境を追加する**

既存環境を参考に、以下の点に注意して追加する：

- `EPD_CS_S_PIN` が必要（slave IC の CS ピン）。既存環境にはない新規フラグ
- `EPD_DC_PIN` はこのドライバでは**使用しない**が、GPIO の競合を避けるためピン番号は割り当てること
- SPI クロックは 4MHz（他デバイスと同じ設定）で開始し、問題があれば下げる

**2. `sendImageData` の行間タイミングを確認する**

サンプルコードは 300 バイト（1 行）ごとに `delay(1ms)` を挿入しているが、
ストリーミング実装では BUF_SIZE（1000 バイト）ごとになっている。
表示が乱れる場合は行間 delay の追加を検討すること。

**3. Web API 側の対応を確認する**

`/pf/{MAC}/image/random.bin` が 13in3e 向けに以下の形式でデータを返すこと：
- 前半 480,000 バイト: 左パネル（CS_M）用データ（300 バイト/行 × 1600 行）
- 後半 480,000 バイト: 右パネル（CS_S）用データ（300 バイト/行 × 1600 行）
