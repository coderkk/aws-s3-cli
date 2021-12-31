#!/bin/bash

# ########################################################## #
# Makefile for Golang Project
# Includes cross-compiling, installation, cleanup
# ########################################################## #

# Check for required command tools to build or stop immediately
EXECUTABLES = git go find
K := $(foreach exec,$(EXECUTABLES),\
        $(if $(shell which $(exec)),some string,$(error "No $(exec) in PATH")))

# The binary to build (just the basename).
BIN ?= qlauncher

# This repo's root import path (under GOPATH).
PKG=github.com/poseidon.network/qlauncher

ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

PACKAGE_NAME=mineral-cli
VERSION=1.0.1
BUILD=`git rev-parse HEAD`
PLATFORMS=linux
ARCHITECTURES=arm64 amd64

BUILD_DIR=build/
BUILD_FILE=$(BUILD_DIR)$(PACKAGE_NAME)

# Setup linker flags option for build that interoperate with variable names in src code
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.Build=${BUILD}"

# default: compile

all: clean compile_all

compile:
	@go build ${LDFLAGS}  -o ${BUILD_FILE}

compile_all:
	$(foreach GOOS, $(PLATFORMS),\
	$(foreach GOARCH, $(ARCHITECTURES), $(shell export GOOS=$(GOOS); export GOARCH=$(GOARCH); go build -v -o $(BUILD_FILE)-$(GOOS)-$(GOARCH))))
	$(shell export GOOS=linux; export GOARCH=arm; export GOARM=5; go build -v -o $(BUILD_FILE)-linux-armhf)

clean:
	@find ${ROOT_DIR}/${BUILD_DIR} -name '${PACKAGE_NAME}[-?][a-zA-Z0-9]*[-?][a-zA-Z0-9]*' -delete

run:
	${BUILD_DIR}/qlauncher
