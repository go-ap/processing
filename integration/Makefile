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
	go test $(TEST_FLAGS) -json ./ -verbose -inbox -outbox | go tool tparse -all -progress -trimpath github.com/go-ap/

coverage:
	go tool covdata percent -i="$(find ./ -iname "coverage" -exec ls -d {} + | tr ' ' ',')"
