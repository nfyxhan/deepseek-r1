all: win mac linux-amd64 linux-arm64

win:
	GOOS=windows GOARCH=amd64 go build -o bin/deepseek-amd64.exe main.go

mac:
	GOOS=darwin go build -o bin/deepseek-darwin main.go

linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o bin/deepseek-linux-amd64 main.go

linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o bin/deepseek-linux-arm64 main.go
