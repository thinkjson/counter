build: amd64 arm64 darwin
dev:
	find . -type f \( -name '*.go' -o -path './ui/*' \) | entr -r -d go run .
amd64:
	env GOOS=linux GOARCH=amd64 go build -o system_collector_amd64 system_collector.go
arm64:
	env GOOS=linux GOARCH=arm64 go build -o system_collector_arm64 system_collector.go
darwin:
	env GOOS=darwin GOARCH=amd64 go build -o system_collector_darwin system_collector.go