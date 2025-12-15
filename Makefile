test:
	@go test ./...

lint:
	@golangci-lint run ./...

format:
	@golangci-lint fmt

release-clean:
	rm -rf dist

release-build-mac-x64:
	env GOOS=darwin GOARCH=amd64 go build -o dist/darwin/amd64/cryptonabber-txn-sync cmd/main.go
	tar -C dist/darwin/amd64/ -czvf dist/darwin/amd64/osx-x64.tar.gz cryptonabber-txn-sync

release-build-mac-arm64:
	env GOOS=darwin GOARCH=arm64 go build -o dist/darwin/arm64/cryptonabber-txn-sync cmd/main.go
	tar -C dist/darwin/arm64/ -czvf dist/darwin/arm64/osx-arm64.tar.gz cryptonabber-txn-sync
release-build-win-x64:
	env GOOS=windows GOARCH=amd64 go build -o dist/windows/amd64/cryptonabber-txn-sync.exe cmd/main.go
	(cd dist/windows/amd64 && zip -r - cryptonabber-txn-sync.exe) > dist/windows/amd64/win-x64.zip

release-build: release-build-mac-x64 release-build-mac-arm64 release-build-win-x64

release: release-clean release-build