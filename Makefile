# Monorepo root. Each deployment owns its targets in cmd/<name>/Makefile,
# pulled in below, so adding a deployment never requires editing this file.
MODULES := $(patsubst %/go.mod,%,$(wildcard core/go.mod services/*/go.mod cmd/*/go.mod))

.PHONY: tidy

# go mod tidy each module against its own go.mod (not the workspace).
tidy:
	@for m in $(MODULES); do echo "tidy $$m"; (cd $$m && GOWORK=off go mod tidy) || exit 1; done

include $(wildcard cmd/*/Makefile)
