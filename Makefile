.PHONY: build run tidy migrate

build:
	CGO_ENABLED=0 go build -o /opt/waf/bin/waf ./cmd/waf

run:
	go run ./cmd/waf

tidy:
	go mod tidy

migrate:
	# placeholder
