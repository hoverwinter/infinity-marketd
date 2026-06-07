CONFIG ?= configs/config.yaml
LISTEN ?= 127.0.0.1:8808
URL ?= http://$(LISTEN)

.PHONY: help test build serve querier-serve querier-health console-install console-dev console-build console-check console-serve

help:
	@echo "Targets:"
	@echo "  make serve          Start infinity querier server"
	@echo "  make querier-serve  Start infinity querier server"
	@echo "  make querier-health Check querier health"
	@echo "  make console-install Install console Node dependencies"
	@echo "  make console-dev     Start Vite console dev server"
	@echo "  make console-build   Build console static assets"
	@echo "  make console-check   Type-check console frontend"
	@echo "  make console-serve   Start standalone infinity-console"
	@echo "  make test           Run Go tests"
	@echo "  make build          Build binaries"
	@echo ""
	@echo "Variables:"
	@echo "  CONFIG=$(CONFIG)"
	@echo "  LISTEN=$(LISTEN)"
	@echo "  URL=$(URL)"

serve: querier-serve

querier-serve:
	go run ./cmd/infinity querier serve --config $(CONFIG) --listen $(LISTEN)

querier-health:
	go run ./cmd/infinity querier health --url $(URL)

test:
	go test ./...

build:
	go build ./cmd/infinity ./cmd/marketd ./cmd/infinity-console

console-install:
	npm --prefix web/console install

console-dev:
	npm --prefix web/console run dev

console-build:
	npm --prefix web/console run build

console-check:
	npm --prefix web/console run check

console-serve:
	go run ./cmd/infinity-console --config $(CONFIG) --console-dist web/console/dist
