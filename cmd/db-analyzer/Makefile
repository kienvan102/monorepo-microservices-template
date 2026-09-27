# db-analyzer targets, included by the root Makefile: paths are relative to
# the repo root. Every included cmd/*/Makefile shares one flat make
# namespace, so targets are prefixed db-analyzer- and defaults are
# target-specific (they apply only to db-analyzer-* targets, and a value
# given on the command line still overrides them).
db-analyzer-%: CONFIG_FILE ?= cmd/db-analyzer/config.yaml
db-analyzer-%: ENV_FILE ?= cmd/db-analyzer/.env

.PHONY: db-analyzer-build db-analyzer-image db-analyzer-check-connection db-analyzer-list db-analyzer-inspect db-analyzer-collect db-analyzer-run

# GOWORK=off: build against this deployment's own go.mod, exactly like the image.
db-analyzer-build:
	cd cmd/db-analyzer && GOWORK=off go build -o ../../bin/db-analyzer .

db-analyzer-image:
	docker build -f cmd/db-analyzer/Dockerfile -t db-analyzer .

db-analyzer-check-connection: db-analyzer-build
	bin/db-analyzer -config "$(CONFIG_FILE)" -env-file "$(ENV_FILE)" -action check

db-analyzer-list: db-analyzer-build
	bin/db-analyzer -config "$(CONFIG_FILE)" -env-file "$(ENV_FILE)" -action list

db-analyzer-inspect: db-analyzer-build
	bin/db-analyzer -config "$(CONFIG_FILE)" -env-file "$(ENV_FILE)" -action inspect

db-analyzer-collect: db-analyzer-build
	bin/db-analyzer -config "$(CONFIG_FILE)" -env-file "$(ENV_FILE)" -action collect

db-analyzer-run: db-analyzer-build
	@test -n "$(SCRIPT)" || (echo 'Set SCRIPT=<path>, for example SCRIPT=common/indexes' >&2; exit 1)
	bin/db-analyzer -config "$(CONFIG_FILE)" -env-file "$(ENV_FILE)" -action run -script "$(SCRIPT)"
