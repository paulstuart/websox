
build:
	GOOS=linux GOARCH=amd64 GOARM=6 CGO_ENABLED=0 go build -v -buildmode=exe -ldflags '-s -w -extldflags "-static"'

manifest:
	manifest.sh

deploy:	build manifest
	cf push

phony:	build deploy manifest
