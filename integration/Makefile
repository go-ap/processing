SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

.PHONY: test

test: TEST_FLAGS ?= -timeout 180s -count=1 -covermode=count -coverpkg github.com/go-ap/processing
test:
	go test  -json -cover $(TEST_FLAGS) ./ | go tool tparse -all -progress
