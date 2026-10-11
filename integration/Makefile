SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

TEST_FLAGS ?= -timeout 180s -count=1

ifeq ($(origin GOCOVERDIR),undefined)
TEST_FLAGS += -test.artifacts
endif

.PHONY: test coverage

test:
	go test $(TEST_FLAGS) -json ./ -verbose -c2s -s2s | go tool tparse -all -progress -trimpath github.com/go-ap/

coverage:
	go tool covdata percent -i="$(find ./ -iname "coverage" -exec ls -d {} + | tr ' ' ',')"
