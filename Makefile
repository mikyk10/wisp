SHELL        := /bin/bash
SAMPLE_COUNT ?= 100

SEDOPT := -i
ifeq ($(shell uname), Darwin)
  SEDOPT := -i ''
endif

-include .env
PHOTO_DIR ?= $(HOME)/Pictures/wisp

.PHONY: dev-setup
## Initial setup for local development.
## Usage: make dev-setup
dev-setup:
	@echo "==> [1/3] api/config/ files"
	@if [ ! -f api/config/config.yaml ]; then \
		cp api/config/config.yaml.example api/config/config.yaml; \
		echo "  created: api/config/config.yaml"; \
		echo ""; \
		echo "  Database driver:"; \
		echo "    1) sqlite — no extra setup needed (recommended for local dev)"; \
		echo "    2) mysql  — requires an existing MySQL / MariaDB instance"; \
		read -r -p "  Choice [1]: " db_choice; \
		case "$${db_choice:-1}" in \
			2) \
				default_dsn="user:pass@tcp(localhost:3306)/wspf?charset=utf8mb4&parseTime=True&loc=Local"; \
				read -r -p "  MySQL DSN [$$default_dsn]: " db_dsn; \
				db_dsn=$${db_dsn:-$$default_dsn}; \
				sed $(SEDOPT) "s|driver: sqlite|driver: mysql|g" api/config/config.yaml; \
				sed $(SEDOPT) "s|dsn: /data/wspf.db|dsn: \"$$db_dsn\"|g" api/config/config.yaml; \
				echo "  → driver=mysql";; \
			*) \
				echo "  → driver=sqlite";; \
		esac; \
	else \
		echo "  skipped: api/config/config.yaml (already exists)"; \
	fi

	@echo ""
	@if [ ! -f api/config/service.yaml ]; then \
		cp api/config/service.yaml.example api/config/service.yaml; \
		echo "  created: api/config/service.yaml"; \
		echo ""; \
		read -r -p "  Display MAC address (or 'dev' for local testing without a device) [dev]: " mac; \
		mac=$${mac:-dev}; \
		sed $(SEDOPT) "s|mac_address: dev|mac_address: $$mac|g" api/config/service.yaml; \
		echo ""; \
		echo "  e-Paper model:"; \
		echo "    1) ws7in3e  — 7.3\"  E6 full color (recommended)"; \
		echo "    2) ws7in3f  — 7.3\"  7-color (longer refresh)"; \
		echo "    3) ws4in0e  — 4.0\"  6-color"; \
		echo "    4) ws13in3e — 13.3\" E6 full color"; \
		echo "    5) ws13in3k — 13.3\" 4 grayscale"; \
		read -r -p "  Choice [1]: " model_choice; \
		case "$${model_choice:-1}" in \
			2) model=ws7in3f;; \
			3) model=ws4in0e;; \
			4) model=ws13in3e;; \
			5) model=ws13in3k;; \
			*) model=ws7in3e;; \
		esac; \
		sed $(SEDOPT) "s|model: ws7in3e|model: $$model|g" api/config/service.yaml; \
		echo ""; \
		echo "  Display orientation:"; \
		echo "    1) landscape (default)"; \
		echo "    2) portrait"; \
		read -r -p "  Choice [1]: " orient_choice; \
		case "$${orient_choice:-1}" in \
			2) orient=portrait;; \
			*) orient=landscape;; \
		esac; \
		sed $(SEDOPT) "s|orientation: landscape|orientation: $$orient|g" api/config/service.yaml; \
		echo "  → mac=$$mac  model=$$model  orientation=$$orient"; \
	else \
		echo "  skipped: api/config/service.yaml (already exists)"; \
	fi

	@echo ""
	@echo "==> [2/3] Configure .env"
	@if [ -f .env ]; then \
		echo "  skipped: .env already exists"; \
	else \
		echo ""; \
		read -r -p "  Photo directory [$(PHOTO_DIR)]: " input; \
		photo_dir=$${input:-$(PHOTO_DIR)}; \
		echo ""; \
		echo "  API_BASE_URL is the address the browser uses to reach the API."; \
		echo "  Use the default if you access from this machine (localhost)."; \
		echo "  Enter your LAN IP if accessing from another device on the network."; \
		read -r -p "  API base URL [http://localhost:9002]: " api_url; \
		api_url=$${api_url:-http://localhost:9002}; \
		printf 'PHOTO_DIR=%s\nAPI_BASE_URL=%s\n' "$$photo_dir" "$$api_url" > .env; \
		echo ""; \
		echo "  created: .env"; \
		echo "    PHOTO_DIR=$$photo_dir"; \
		echo "    API_BASE_URL=$$api_url"; \
	fi; \
	photo_dir=$$(grep '^PHOTO_DIR=' .env | cut -d= -f2-); \
	\
	echo ""; \
	echo "==> [3/3] Sample images"; \
	mkdir -p "$$photo_dir"; \
	if [ -z "$$(ls -A "$$photo_dir" 2>/dev/null)" ]; then \
		read -r -p "  $$photo_dir is empty. Download $(SAMPLE_COUNT) sample images from picsum.photos? [y/N]: " yn; \
		case "$$yn" in \
			[yY]*) \
				echo "  Downloading $(SAMPLE_COUNT) images..."; \
				for i in $$(seq 1 $(SAMPLE_COUNT)); do \
					curl -sL "https://picsum.photos/seed/$$i/1600/1200" \
						-o "$$photo_dir/sample_$$(printf '%03d' $$i).jpg"; \
					printf "."; \
				done; \
				echo ""; \
				echo "  done: $$(ls "$$photo_dir" | wc -l | tr -d ' ') files in $$photo_dir";; \
			*) \
				echo "  Skipped. Put your photos in $$photo_dir before starting.";; \
		esac; \
	else \
		echo "  $$photo_dir already has files, skipping download."; \
	fi; \
	\
	echo ""; \
	echo "Done. Next:"; \
	echo "  1. docker compose up --build"; \
	echo "  2. make scan          # index photos into the DB"

.PHONY: up
## Start the stack. scan runs automatically before api starts.
up:
	docker compose up --build

.PHONY: samples
## Download sample images from picsum.photos into PHOTO_DIR.
## Usage: make samples [PHOTO_DIR=/path/to/photos] [SAMPLE_COUNT=20]
samples:
	@mkdir -p "$(PHOTO_DIR)"
	@echo "Downloading $(SAMPLE_COUNT) sample images into $(PHOTO_DIR) ..."
	@for i in $$(seq 1 $(SAMPLE_COUNT)); do \
		curl -sL "https://picsum.photos/seed/$$i/1600/1200" \
			-o "$(PHOTO_DIR)/sample_$$(printf '%03d' $$i).jpg"; \
		printf "."; \
	done
	@echo ""
	@echo "Done: $$(ls "$(PHOTO_DIR)" | wc -l | tr -d ' ') files in $(PHOTO_DIR)"
