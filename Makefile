CONFIG ?= examples/config.example.yaml
LISTEN ?= 127.0.0.1:8808
URL ?= http://$(LISTEN)

.PHONY: help test build serve querier-serve querier-health

help:
	@echo "Targets:"
	@echo "  make serve          Start infinity querier server"
	@echo "  make querier-serve  Start infinity querier server"
	@echo "  make querier-health Check querier health"
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
	go build ./cmd/infinity ./cmd/marketd
