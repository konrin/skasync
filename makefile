build:
	go build -o out/skasync cmd/skasync/*.go

compile:
	# MacOS
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o out/skasync-darwin-amd64 cmd/skasync/*.go
	# Linux
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o out/skasync-linux-amd64 cmd/skasync/*.go
	# Windows
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o out/skasync-windows-amd64.exe cmd/skasync/*.go
