PATH := $(PATH):$(PWD)

TEMPLATE     ?= "manifest.yml.m4"
APP_BUILD    ?= $(shell date +'%Y%m%d.%H%M.%S')
APP_VERSION  ?= $(shell git log -n 1 --pretty=format:%H)
APP_LOGLEVEL ?= debug
APP_ENV      ?= $(shell cf target | awk '/^space:/ {print $2}' | sed -e 's/^.*\-//') # awk can do regex but it diff
APP_FILE     ?= $(shell echo ${APP_ENV} | sed -e s/$$/.env/) # the thing we do to concatenate in a Makefile
APP_EXE      ?= $(shell basename $$PWD)
APP_MEM      ?= 256M
APP_DISK     ?= 256M
APP_COUNT    ?= 1

CGO_ENABLED  ?=0

M4_ARGS	= -DAPP_ENV=$(APP_ENV) \
	  -DAPP_EXE=$(APP_EXE) \
	  -DAPP_VERSION=$(APP_VERSION) \
	  -DAPP_LOGLEVEL=$(APP_LOGLEVEL) \
	  -DAPP_COUNT=$(APP_COUNT) \
	  -DAPP_BUILD=$(APP_BUILD) \
	  -DAPP_DISK=$(APP_DISK) \
	  -DAPP_MEM=$(APP_MEM) \
          -DUAA_URL=$(uaa_url) \
          -DUAA_CLIENT_ID=$(uaa_client_id) \
          -DUAA_CLIENT_SECRET=$(uaa_client_secret)
build:
	GOOS=linux GOARCH=amd64 GOARM=6 CGO_ENABLED=0 go build -v -buildmode=exe -ldflags '-s -w -extldflags "-static"'

manifest:
	@m4 $(M4_ARGS) $(TEMPLATE) > manifest.yml

deploy:	build manifest
	cf push

phony:	build deploy manifest
