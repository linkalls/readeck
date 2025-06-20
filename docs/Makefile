# SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

UV=$(shell command -pv uv)

GO ?= go
FILE_COMPOSE_PKG ?= codeberg.org/readeck/file-compose@latest

all: compile

# Extract all the messages
extract:
ifneq ($(UV),)
	uv run messages.py extract
else
	@echo "ERROR: uv not found"
	@false
endif

# Update po files
update:
ifneq ($(UV),)
	uv run messages.py update
else
	@echo "ERROR: uv not found"
	@false
endif

# Generate markdown from po files.
generate:
ifneq ($(UV),)
	$(UV) run messages.py generate
else
	@echo "WARNING: uv not found, not compiling locales"
endif

check:
ifneq ($(UV),)
	$(UV) run messages.py check
else
	@echo "WARNING: uv not found, not compiling locales"
endif

build: generate
	$(GO) run $(FILE_COMPOSE_PKG) -format json api/api.yaml assets/api.json
	$(GO) run ../tools/docs src assets

clean:
	rm -rf assets/*
	find src -name '*.md' -not -regex "src/en-US/.*" -delete
