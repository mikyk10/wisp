# AI Pipeline 実装仕様

main ブランチから新規ブランチを派生し、以下の仕様に基づいて AI パイプライン基盤を実装する。
既存の `feature/ai-tagging` ブランチのコードは参照しない。ゼロから構築する。

## 背景

WiSP は e-Paper フォトフレーム向けの画像配信サーバー。Go (Echo/GORM/SQLite) バックエンドと Vue SPA フロントエンドから成る。
カタログ（画像コレクション）単位で画像を管理し、フレームからのリクエストに対してランダムに1枚選んで配信する。

本実装では、2種類の AI パイプラインを追加する:
1. **タグ付けパイプライン** — 既存のファイルカタログ画像を VLM で説明し、タグを抽出する
2. **画像生成パイプライン** — LLM + 画像生成 API でフォトフレーム向け画像を生成し、キャッシュする

両パイプラインは共通のパイプラインランナー基盤の上に構築する。

## テーブル設計

### pipeline_executions — パイプライン実行単位

1回のパイプライン実行（= 1画像に対する全ステージの処理）を表す。

```go
type PipelineExecution struct {
    ID            PrimaryKey       `gorm:"primaryKey;autoIncrement"`
    PipelineType  string           `gorm:"type:varchar(32);not null;index"`           // "tagging" | "generate"
    CatalogKey    string           `gorm:"type:varchar(64);not null;index"`
    SourceImageID *PrimaryKey      `gorm:"index"`                                     // nullable: タグ付け時は対象Image.ID、img2img時はソース画像ID、テキストのみ生成時はNULL
    Status        ExecutionStatus  `gorm:"type:varchar(32);not null"`                 // pending/running/success/failed
    StartedAt     time.Time        `gorm:"not null"`
    FinishedAt    sql.NullTime
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### step_executions — ステップ実行記録

パイプライン内の各ステージの実行を記録する。

```go
type StepExecution struct {
    ID                  PrimaryKey      `gorm:"primaryKey;autoIncrement"`
    PipelineExecutionID PrimaryKey      `gorm:"not null;index"`                       // FK → pipeline_executions
    StageName           string          `gorm:"type:varchar(64);not null"`            // YAML で定義したステージ名
    StageIndex          int             `gorm:"not null"`                             // 0-based 実行順
    ProviderName        string          `gorm:"type:varchar(64)"`                     // provider config key
    ModelName           string          `gorm:"type:varchar(128)"`
    PromptVersion       string          `gorm:"type:varchar(32)"`
    PromptHash          string          `gorm:"type:char(12)"`
    Status              ExecutionStatus `gorm:"type:varchar(32);not null"`
    StartedAt           time.Time       `gorm:"not null"`
    FinishedAt          sql.NullTime
    LatencyMs           int64
    ErrorCode           string          `gorm:"type:varchar(64)"`
    ErrorMessage        string          `gorm:"type:text"`
    CreatedAt           time.Time
    UpdatedAt           time.Time
}
```

### step_outputs — ステップ出力

各ステップの出力を保存する。テキストとバイナリを分離。

```go
type StepOutput struct {
    ID              PrimaryKey `gorm:"primaryKey;autoIncrement"`
    StepExecutionID PrimaryKey `gorm:"not null;uniqueIndex"`                         // FK → step_executions (1:1)
    ContentType     string     `gorm:"type:varchar(64);not null"`                    // "text/plain", "image/png", etc.
    ContentText     *string    `gorm:"type:text"`                                    // テキスト出力（nullable）
    ContentBlob     []byte     `gorm:"type:blob"`                                    // バイナリ出力（nullable）
    CreatedAt       time.Time
}
// CHECK: ContentText IS NOT NULL OR ContentBlob IS NOT NULL
```

### generation_cache — 生成画像キャッシュ

フレーム配信用の最終成果物キャッシュ。

```go
type GenerationCacheEntry struct {
    ID                  PrimaryKey `gorm:"primaryKey;autoIncrement"`
    CatalogKey          string     `gorm:"type:varchar(64);not null;index:idx_cache_random,priority:1"`
    PipelineExecutionID PrimaryKey `gorm:"not null;uniqueIndex"`                     // FK → pipeline_executions (cascade delete)
    ImageData           []byte     `gorm:"type:blob;not null"`
    ContentType         string     `gorm:"type:varchar(64);not null;default:'image/png'"`
    Rnd                 float64    `gorm:"type:double;not null;index:idx_cache_random,priority:2"` // ランダム選択用
    CreatedAt           time.Time
}
```

### tags — タグマスタ

```go
type Tag struct {
    ID             PrimaryKey `gorm:"primaryKey;autoIncrement"`
    NameNormalized string     `gorm:"type:varchar(128);not null;uniqueIndex"`
    DisplayName    string     `gorm:"type:varchar(128);not null"`
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

### image_tags — 画像-タグ紐付け

```go
type ImageTag struct {
    ImageID      PrimaryKey `gorm:"primaryKey"`
    TagID        PrimaryKey `gorm:"primaryKey"`
    SourceStepID PrimaryKey `gorm:"not null;index"`                                  // FK → step_executions（どのステップが生成したか）
    Score        *float64   `gorm:"type:double"`
    CreatedAt    time.Time
}
```

### ステータス定義

```go
type ExecutionStatus string

const (
    StatusPending ExecutionStatus = "pending"
    StatusRunning ExecutionStatus = "running"
    StatusSuccess ExecutionStatus = "success"
    StatusFailed  ExecutionStatus = "failed"
)
```

### CASCADE ルール

- `pipeline_executions` 削除時 → 配下の `step_executions`, `step_outputs`, `generation_cache` を CASCADE DELETE
- `generation_cache` エントリのエビクション時 → 紐づく `pipeline_executions` を削除 → CASCADE で中間成果物も削除

## YAML 設定構造

### service.yaml（カタログ定義）

```yaml
catalog:
  # ファイルカタログ（既存）
  - key: photos
    type: file
    file:
      src_path: /mnt/wisp

  # 画像生成カタログ（新規）
  - key: ai-landscapes
    type: generate
    generate:
      cache_depth: 10          # キャッシュに保持する最大枚数
      evict_count: 3           # バッチ実行時にFIFOで追い出す枚数
      pipeline:
        stages:
          - name: meta-prompt
            output: text
            prompt: prompts/landscape_meta.md
          - name: generate
            output: image
            prompt: prompts/landscape_gen.md

  # img2img 画像生成カタログ（ソースカタログあり）
  - key: ai-stylized
    type: generate
    generate:
      cache_depth: 5
      evict_count: 2
      source_catalog: photos     # img2img 入力元カタログ
      pipeline:
        stages:
          - name: describe
            output: text
            prompt: prompts/describe.md
            image_input: $source   # source_catalog から取得した画像
          - name: stylize
            output: image
            prompt: prompts/stylize.md
            image_input: $source

  # AI生成お気に入り保存先（通常のファイルカタログ）
  - key: ai-favorites
    type: file
    file:
      src_path: /mnt/wisp/ai-favorites
```

### global config（AI セクション）

```yaml
ai:
  providers:
    ollama_local:
      endpoint: http://localhost:11434/v1
    openai:
      endpoint: https://api.openai.com/v1
      api_key: ${OPENAI_API_KEY}
  workers: 2                      # env: WISP_AI_WORKERS
  request_timeout_sec: 120        # env: WISP_AI_REQUEST_TIMEOUT_SEC
  max_retries: 3                  # env: WISP_AI_MAX_RETRIES
```

### タグ付けパイプライン設定

タグ付けはカタログタイプではなく、ファイルカタログに対する処理として設定する。

```yaml
ai:
  tagging:
    max_tags: 15                  # env: WISP_AI_MAX_TAGS
    pipeline:
      stages:
        - name: descriptor
          output: text
          prompt: prompts/descriptor_v1.md
          image_input: $source    # 対象画像のサムネイル
        - name: tagger
          output: text
          prompt: prompts/tagger_v1.md
```

## プロンプトファイルフォーマット

YAML フロントマター + Markdown 本文。本文はテキスト専用（画像参照は YAML の `image_input` で宣言）。

```markdown
---
version: v1
stage: descriptor
provider: ollama_local
model: qwen3.5:9b
temperature: 0.3
max_tokens: 500
---
Describe this photo in detail. Focus on subjects, setting, mood, colors, and objects.
Write in plain English. Write in plain prose paragraphs.
```

### 画像生成ステージのプロンプト

`output: image` のステージは `api_type` によって呼び出す API が異なる。

**chat completion 経由の画像生成** (`api_type: chat`):
GPT-4o 等が chat completion レスポンスの content parts に画像を含めて返すパターン。
AnyLLM / openai-go は chat completion レスポンスの画像パートをハンドルできないため、
**このルートのみ raw HTTP で chat completion API を呼び、レスポンス JSON から画像を自前で抽出する**。

```markdown
---
version: v1
stage: stylize
provider: openai
model: gpt-4o
api_type: chat
---
この画像をスタジオジブリの水彩画風に変換してください。元の構図と被写体を維持してください。
```

テキストから画像を生成する場合:

```markdown
---
version: v1
stage: generate
provider: openai
model: gpt-4o
api_type: chat
temperature: 1.0
---
{{.prev.output}}

上記のコンセプトに基づいて、e-Paper フォトフレームに適した画像を生成してください。
```

**images API 経由の画像生成** (`api_type: image_generation`):
DALL-E 3 等の `/v1/images/generations` エンドポイントを使う。テキスト→画像のみ（img2img 不可）。

```markdown
---
version: v1
stage: generate
provider: openai
model: dall-e-3
api_type: image_generation
size: 1024x1024
quality: standard
---
{{.prev.output}}
```

### テンプレート変数

プロンプト本文で利用可能な変数:

| 変数 | 説明 |
|---|---|
| `{{.prev.output}}` | 直前ステージのテキスト出力 |
| `{{.stages.STAGE_NAME.output}}` | 指定ステージのテキスト出力 |
| `{{.config.MaxTags}}` | 設定値の参照（タグ付け用） |

画像は変数として参照しない。`image_input` フィールドで宣言し、ランタイムが API メッセージに添付する。

## ステージ定義のフィールド

```yaml
- name: stage-name          # 必須: 一意なステージ名（英数字 + ハイフン）
  output: text | image       # 必須: 出力タイプ
  prompt: path/to/prompt.md  # 必須: プロンプトファイルパス
  image_input: reference     # 任意: 画像入力の参照先
```

### `api_type` （プロンプト frontmatter）

| api_type | 使う API | ライブラリ | 用途 |
|---|---|---|---|
| `chat` | `/v1/chat/completions` | `output: text` → AnyLLM, `output: image` → raw HTTP | テキスト生成、chat completion 経由の画像生成 (GPT-4o) |
| `image_generation` | `/v1/images/generations` | openai-go | DALL-E 3 テキスト→画像 |
| `comfyui` | ComfyUI API | （将来実装） | ローカル img2img 等 |

デフォルト値:
- `output: text` → `api_type: chat`
- `output: image` → `api_type: chat`

`api_type` を省略した場合は上記デフォルトが適用される。`image_generation` や `comfyui` を使う場合は明示指定が必要。

### `image_input` の参照ルール

| 値 | 意味 |
|---|---|
| `$source` | パイプラインの入力画像（タグ付け: 対象画像のサムネイル、img2img: source_catalog から取得した画像） |
| `{stage_name}` | 指定ステージの画像出力（`output: image` のステージのみ参照可能） |

`$` プレフィックスはパイプラインレベル入力を示す。ステージ名に `$` は使用不可（バリデーションで弾く）。

## パイプライン実行モデル

### 共通パイプラインランナー

両パイプラインに共通するコア処理:

1. ステージ定義を順に実行
2. 各ステージ: `step_executions` レコード作成 → LLM/画像生成 API 呼び出し → `step_outputs` 保存 → ステータス更新
3. ステージ間のデータ受け渡し: ランナーが全ステージの出力をメモリに保持し、テンプレート変数と `image_input` を解決
4. エラー時: ステップを failed にし、パイプライン実行も failed にして中断
5. 全ステージ成功: パイプライン実行を success にし、ファイナライザを呼ぶ

### ファイナライザ（ユースケース固有のポスト処理）

パイプライン完了後の処理。ランナーには組み込まず、ユースケース層で呼び出す。

- **タグ付けファイナライザ**: 最終ステージのテキスト出力をパースし、Tag + ImageTag レコードを作成
- **画像生成ファイナライザ**: 最終ステージの画像出力（最後の `output: image` ステージ）を `generation_cache` に保存

### ワーカープール

- セマフォベースの並行処理（configurable worker 数）
- 各画像/生成タスクを goroutine で処理

### シグナルハンドリング（CTRL+C 対応）

ラベル付き break で for ループを正しく抜ける:

```go
loop:
    for _, task := range tasks {
        select {
        case <-ctx.Done():
            break loop
        default:
        }
        // dispatch task...
    }
```

コンテキストキャンセル時:
- 新規タスクのディスパッチを停止
- 実行中のワーカーは API 呼び出しのコンテキストキャンセルで中断
- 中断されたステップは `step_executions` を failed に更新してから return（ゴーストレコードを残さない）

## CLI コマンド

### タグ付け

```bash
# パイプライン実行
catalog tagging run --catalog=photos [--workers=4] [--limit=100] [--rebuild] [--dry-run] [--verbose]

# タグ付けデータのリセット
catalog tagging reset --catalog=photos [--image-id=42]
```

タグ付けの `--rebuild`:
- 全画像を再処理する
- 成功済みの descriptor 出力はキャッシュから再利用可能（descriptor ステージの出力が step_outputs に残っていれば）

### 画像生成

```bash
# バッチ生成実行
catalog generate run --catalog=ai-landscapes [--source-id=42] [--workers=2] [--dry-run] [--verbose]

# キャッシュ一覧
catalog generate list --catalog=ai-landscapes

# 失敗した実行のクリーンアップ
catalog generate clean --catalog=ai-landscapes [--failed]

# お気に入りとしてファイルカタログにエクスポート
catalog generate favorite --catalog=ai-landscapes --cache-id=7 --dest=ai-favorites
```

### バッチ生成のフロー

`catalog generate run --catalog=ai-landscapes` 実行時:

1. 現在のキャッシュ枚数を確認
2. キャッシュが cache_depth 以上 → FIFO で evict_count 枚を追い出す（CASCADE で中間成果物も削除）
3. 空きスロット数を計算: `to_generate = cache_depth - current_count`
4. `to_generate` 回、パイプラインを実行:
   a. source_catalog がある場合: ランダムに1枚選択（`--source-id` 指定時はその画像）
   b. pipeline_execution レコード作成
   c. パイプラインランナーでステージを順次実行
   d. 成功 → 画像生成ファイナライザ → generation_cache に保存
   e. 失敗 → pipeline_execution を failed に。バッチ末尾で失敗分をクリーンアップ
5. 完了レポート出力

### お気に入りエクスポートのフロー

`catalog generate favorite --catalog=ai-landscapes --cache-id=7 --dest=ai-favorites` 実行時:

1. generation_cache から ID=7 のエントリを取得
2. 画像データを dest カタログの src_path に PNG ファイルとして書き出す
3. ファイル名: `generated_{cache_id}_{timestamp}.png`
4. dest カタログの次回インデックス時に Image テーブルに取り込まれる

## プロバイダ抽象

### インターフェース

```go
// StageExecutor はパイプラインの1ステージを実行する。
// output タイプと api_type の組み合わせに応じた実装が選ばれる。
type StageExecutor interface {
    Execute(ctx context.Context, prompt string, images [][]byte) (*StageResult, error)
}
```

`StageResult` はテキストまたは画像（または両方）を持つ。ランナーは `output` フィールドに応じて適切なフィールドを使う。

### 実装の分岐

| output | api_type | 実装 | 備考 |
|---|---|---|---|
| `text` | `chat` | AnyLLM (`Completion`) | 既存の chat completion。レスポンスから `ContentString()` でテキスト抽出 |
| `image` | `chat` | **Raw HTTP client** | AnyLLM をバイパス。`/v1/chat/completions` を直接呼び、レスポンス JSON の content parts から base64 画像を抽出。img2img（ジブリ風変換等）はこのルート |
| `image` | `image_generation` | openai-go (`Images.Generate`) | `/v1/images/generations`。DALL-E 3 テキスト→画像。img2img 不可 |
| `image` | `comfyui` | （将来実装） | ComfyUI REST API |

### Raw HTTP client（chat completion 画像抽出）

AnyLLM / openai-go v1.12.0 は chat completion レスポンスの画像パート（content parts 配列）をパースできない
（`ChatCompletionMessage.Content` が `string` 型のため）。このため `output: image` + `api_type: chat` の場合のみ
自前の HTTP クライアントで `/v1/chat/completions` を呼び、レスポンス JSON を直接パースする。

```
リクエスト: 標準的な chat completion リクエスト（messages にテキスト + vision 画像を含む）
レスポンス: choices[0].message.content[] から type="image_url" のパートを探し、
           base64 データまたは URL から画像バイトを取得
```

将来 AnyLLM または openai-go が multi-part レスポンスに対応したら、このルートを AnyLLM 経由に切り替えられる。

### プロバイダ設定

プロンプトファイルの frontmatter `provider` フィールドが、global config の `ai.providers` のキーを参照する。
ランタイムは `output` + `api_type` の組み合わせに応じて適切な `StageExecutor` 実装を構築する。

将来 ComfyUI を追加する場合: `StageExecutor` の新しい実装を追加し、`api_type: comfyui` で選択する。

## カタログプロバイダ（フレーム配信）

`type: generate` カタログ用の新しいプロバイダを実装する。

既存のプロバイダと同じインターフェースで、`generation_cache` テーブルからランダムに1枚選んで返す。
`Image.Rnd` と同じ仕組みで `GenerationCacheEntry.Rnd` を使ったランダム選択を行う。

キャッシュが空の場合:
- フォールバックとしてオンデマンド生成しても良いが、遅いのでログ警告を出す
- 設定で `fallback_generate: true|false` を切り替え可能にする（デフォルト: false → 503 返却）

## 中間成果物のライフサイクル

全ステップの出力（テキスト・画像問わず）を `step_outputs` に保存する。中間ステージの画像出力もデバッグ目的で保持する。

### 画像生成パイプライン

- キャッシュエントリの FIFO エビクション時に CASCADE DELETE で自動削除（紐づく pipeline_execution → step_executions → step_outputs が一括削除）
- 失敗した実行: バッチ末尾で自動クリーンアップ、または `catalog generate clean --failed` で手動

### タグ付けパイプライン

- `catalog tagging reset` で明示的に削除
- descriptor の step_output はキャッシュとして再利用可能（rebuild 時に descriptor ステージをスキップする判断に使う）

## スコープ外（実装しないもの）

- フロントエンド（Vue SPA）の変更
- cron / スケジューラの実装（k8s CronJob で対応）
- プロンプトの変数システム（季節、時間帯など。temperature による自然な発散で対応）
- DAG 実行（ステージは直列チェーンのみ。ただしテンプレート変数で任意の先行ステージのテキスト出力を参照可能）
- バッチ実行テーブル（pipeline_executions の集合で表現。専用テーブルは不要）
- `api_type: comfyui` の実装（将来対応。`api_type` フィールドと `StageExecutor` インターフェースで拡張ポイントは確保済み）

## 技術的制約メモ

### AnyLLM / openai-go の画像レスポンス非対応（2026-03 時点）

openai-go v1.12.0 の `ChatCompletionMessage.Content` は `string` 型のため、
GPT-4o が chat completion レスポンスの content parts に画像を含めて返すケースをパースできない。
AnyLLM はこの openai-go に依存しているため同様の制約を受ける。

対策: `output: image` + `api_type: chat` の場合のみ raw HTTP で `/v1/chat/completions` を呼び、
レスポンス JSON を自前でパースする。将来ライブラリが対応したら切り替え可能。
