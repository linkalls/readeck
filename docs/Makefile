# SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

UV=$(shell command -pv uv)

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
