BINARY := wspf
BASE_PACKAGE=wspf/app/domain/model
ENTRYPOINT=main.go

.DEFAULT_GOAL := build

.EXPORT_ALL_VARIABLES:
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

SHELL := /bin/bash


OPEN := ls -ls
ifeq ($(shell uname) , Darwin)
  # macOS
  # Mac sedはバックアップファイル指定が必要
  $(info Running on macOS)
  SEDOPT := -i '.old'
  OPEN := open

else ifneq ($(shell echo $(WSL_DISTRO_NAME)),)
  # WSL2
  $(info Running on WSL2)
  OPEN := start

else
  # Linux
  SEDOPT := -i
  OPEN := xdg-open
  $(info Running on Linux)

endif


PHOTO_DIR ?= $(HOME)/Pictures/wspf
SAMPLE_COUNT ?= 20

## ローカル開発環境の初期セットアップ
## 使い方: make dev-setup [PHOTO_DIR=/path/to/photos]
.PHONY: dev-setup
dev-setup:
	@echo "==> [1/4] config ファイルを生成"
	@if [ ! -f config/config.yaml ]; then \
		printf 'log_level: DEBUG\nport: 9002\ndatabase:\n  driver: sqlite\n  dsn: /data/wspf.db\n  log: true\n' > config/config.yaml; \
		echo "  created: config/config.yaml"; \
	else \
		echo "  skipped: config/config.yaml (already exists)"; \
	fi
	@if [ ! -f config/service.yaml ]; then \
		printf 'catalog:\n  - key: photos\n    type: file\n    file:\n      src_path: /mnt/wspf\n\ndisplays:\n  - name: dev-display\n    mac_address: dev\n    api_version: 2025-03-01\n    model: ws7in3f\n    orientation: landscape\n    color_reduction:\n      type: floysteinberg\n    catalog:\n      - key: photos\n' > config/service.yaml; \
		echo "  created: config/service.yaml"; \
	else \
		echo "  skipped: config/service.yaml (already exists)"; \
	fi
	@echo "==> [2/4] .env を生成 (docker compose 用)"
	@printf 'PHOTO_DIR=$(PHOTO_DIR)\n' > .env
	@echo "  PHOTO_DIR=$(PHOTO_DIR)"
	@echo "==> [3/4] 写真ディレクトリを作成"
	@mkdir -p "$(PHOTO_DIR)"
	@echo "  $(PHOTO_DIR)"
	@echo "==> [4/4] サンプル画像をダウンロード (空の場合のみ)"
	@if [ -z "$$(ls -A "$(PHOTO_DIR)" 2>/dev/null)" ]; then \
		echo "  Downloading $(SAMPLE_COUNT) sample images..."; \
		for i in $$(seq 1 $(SAMPLE_COUNT)); do \
			curl -sL "https://picsum.photos/seed/$${i}/1600/1200" \
				-o "$(PHOTO_DIR)/sample_$$(printf '%03d' $$i).jpg"; \
			printf "."; \
		done; \
		echo ""; \
		echo "  done: $$(ls "$(PHOTO_DIR)" | wc -l | tr -d ' ') files"; \
	else \
		echo "  skipped: $(PHOTO_DIR) already has files"; \
	fi
	@echo ""
	@echo "セットアップ完了。次のコマンドでコンテナを起動してください:"
	@echo "  docker compose up"

require-%:
	@ if [ "$(shell command -v ${*} 2> /dev/null)" = "" ]; then \
		echo "[$*] not found in your PATH"; \
		exit 1; \
	fi

build: $(BINARY)
$(BINARY) : .EXPORT_ALL_VARIABLES 
# GitHub Actionsを利用している場合は、GITHUB_REFからリビジョンとタグ名を取得する
	go build -ldflags "-X $(BASE_PACKAGE).AppVersion=$(shell echo "$${GITHUB_REF_NAME}") \
	-X $(BASE_PACKAGE).CommitHash=$(shell echo "$${GITHUB_SHA:0:7}") \
	-X $(BASE_PACKAGE).BuildTime=$(shell date +'%Y-%m-%dT%H:%M:%S')" \
	-o $(BINARY) $(ENTRYPOINT)

.PHONY: clean
clean:
	rm -rf $(BINARY)

.PHONY: lint-local
lint-local:
	docker run --rm -t -v $$(pwd):/app -v ~/.cache/golangci-lint/v2.0:/root/.cache -w /app golangci/golangci-lint:v2.0 golangci-lint run -v

.PHONY: test
test:
	go test -v -coverpkg=./... ./... -coverprofile=tmp/coverage.out
	go tool cover -html=tmp/coverage.out -o tmp/coverage_report.html
	$(OPEN) tmp/coverage_report.html


## YAML設定ファイルに対応する構造体をJSONスキーマから生成する
.PHONY: struct
struct: require-schematyper
	schematyper app/domain/model/config/raw/schema.json -o app/domain/model/config/raw/config.go --package=raw --root-type=ServiceConfig
#### 生成された構造体にはYAMLタグが打たれていないので、schematyperが付けたJSONタグを元にしてYAMLタグを構造体に追加する
	sed $(SEDOPT) 's/`json:\([^`]*\)`/`json:\1 yaml:\1`/g' app/domain/model/config/raw/config.go
	rm -f app/domain/model/config/raw/config.go.old
