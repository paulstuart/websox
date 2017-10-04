
build:
	GOOS=linux GOARCH=amd64 GOARM=6 CGO_ENABLED=0 go build -v -buildmode=exe -ldflags '-extldflags "-static"'

phony:	build
