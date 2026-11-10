# On macOS every Docker command runs inside the Lima VM. On a Linux host with
# Docker and a BTF-enabled kernel, the same targets run Docker directly.
VM      ?= obi-demo
GATEWAY ?= http://localhost:8080

ifeq ($(shell uname -s),Darwin)
  DOCKER := limactl shell --workdir "$(CURDIR)" $(VM) sudo docker
else
  DOCKER := docker
endif
COMPOSE := $(DOCKER) compose

.PHONY: help vm vm-stop vm-delete vm-shell up down restart ps logs obi-logs validate \
        chaos-latency chaos-errors chaos-off open check-kernel

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

vm: ## Create and start the Lima VM (macOS)
	limactl start --name=$(VM) --tty=false lima/obi-demo.yaml

vm-stop: ## Stop the Lima VM
	limactl stop $(VM)

vm-delete: ## Delete the Lima VM
	limactl delete --force $(VM)

vm-shell: ## Open a shell in the VM, in this directory
	limactl shell --workdir "$(CURDIR)" $(VM)

check-kernel: ## Show the kernel, BTF and tracefs that OBI will use
	$(DOCKER) run --rm --privileged --pid=host debian:trixie-slim sh -c \
	  'uname -r; ls -l /sys/kernel/btf/vmlinux; ls /sys/kernel/tracing >/dev/null && echo tracefs ok'

up: ## Build and start everything
	$(COMPOSE) up -d --build
	@echo
	@echo "  Shop        http://localhost:8080"
	@echo "  Perses      http://localhost:8090"
	@echo "  Jaeger      http://localhost:16686"
	@echo "  Prometheus  http://localhost:9090"

down: ## Stop and remove everything
	$(COMPOSE) down -v

restart: ## Restart OBI (pick up config/obi/obi.yaml changes)
	$(COMPOSE) restart obi

ps: ## Show container status
	$(COMPOSE) ps

logs: ## Follow logs for all services
	$(COMPOSE) logs -f --tail=50

obi-logs: ## Follow OBI's logs
	$(COMPOSE) logs -f --tail=100 obi

validate: ## Validate OBI, Collector, Prometheus and compose configs
	$(DOCKER) run --rm -v "$(CURDIR)/config/obi/obi.yaml:/c.yaml:ro" otel/ebpf-instrument:v1.0.0 config validate /c.yaml
	$(DOCKER) run --rm -v "$(CURDIR)/config/otelcol/config.yaml:/c.yaml:ro" otel/opentelemetry-collector-contrib:0.165.0 validate --config=/c.yaml
	$(DOCKER) run --rm --entrypoint promtool -v "$(CURDIR)/config/prometheus/prometheus.yaml:/p.yaml:ro" prom/prometheus:v3.16.0 check config /p.yaml
	$(COMPOSE) config -q && echo "compose: ok"

chaos-latency: ## Add ~400ms latency to the catalog
	curl -fsS -X POST $(GATEWAY)/admin/chaos/catalog -H 'Content-Type: application/json' -d '{"latency_ms": 400}'; echo

chaos-errors: ## Make 20% of cart requests fail
	curl -fsS -X POST $(GATEWAY)/admin/chaos/cart -H 'Content-Type: application/json' -d '{"error_rate": 0.2}'; echo

chaos-off: ## Remove all injected faults
	curl -fsS -X POST $(GATEWAY)/admin/chaos/catalog -H 'Content-Type: application/json' -d '{"latency_ms": 0, "error_rate": 0}'; echo
	curl -fsS -X POST $(GATEWAY)/admin/chaos/cart -H 'Content-Type: application/json' -d '{"latency_ms": 0, "error_rate": 0}'; echo

open: ## Open the shop, Perses and Jaeger in your browser (macOS)
	open http://localhost:8080 http://localhost:8090 http://localhost:16686
