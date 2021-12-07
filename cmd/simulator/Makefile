
GO = CGO_ENABLED=0 go

VERSION = 0.0.3

BIN = llrp-simulator
TMP = dist/llrp-sim
OUTPUT = dist/llrp-sim-v$(VERSION).tgz

.PHONY: build dist docker fmt

build: $(BIN)

$(BIN): simulator.go $(wildcard ../../internal/llrp/*)
	$(GO) build -o $(BIN) simulator.go

dist: $(OUTPUT)

dist/:
	mkdir -p $@

$(OUTPUT): build README.md $(wildcard configs/*.json) | dist/
	-rm -rf $(TMP)
	mkdir -p $(TMP)
	cp -a configs $(BIN) README.md LICENSE $(TMP)
	cd dist && tar -cvzf $(CURDIR)/$(OUTPUT) llrp-sim
	-rm -rf $(TMP)

docker: build
	docker build -t llrp-sim:$(VERSION) .

fmt:
	$(GO) fmt ./...
