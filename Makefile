#!/usr/bin/env make

GIT_ROOT:=$(shell git rev-parse --show-toplevel)

.PHONY: all clean format lint vet build test tools integration

all: clean format vet build test integration

clean:
	${GIT_ROOT}/make/clean

format:
	${GIT_ROOT}/make/format

vet:
	${GIT_ROOT}/make/vet

build:
	${GIT_ROOT}/make/build

integration:
	${GIT_ROOT}/make/integration
	
test:
	${GIT_ROOT}/make/test

tools:
	${GIT_ROOT}/make/tools

