GO ?= go
TEST := $(GO) test
TEST_FLAGS ?= -v
TEST_TARGET ?= ./...
GO111MODULE = on
PROJECT := $(shell basename $(PWD))

ifeq ($(origin GOCOVERDIR),undefined)
GOCOVERDIR := $(shell pwd)/_coverage
endif

.PHONY: test coverage clean download

download: go.sum

go.sum: go.mod
	$(GO) mod tidy

test: TEST_FLAGS += -json -cover -coverpkg github.com/go-ap/processing -covermode=count
test: go.sum
	$(TEST) $(TEST_FLAGS) $(TEST_TARGET) | go tool tparse -all

coverage: TEST_FLAGS += -coverpkg github.com/go-ap/processing -covermode=count -args -test.gocoverdir=$(GOCOVERDIR)
coverage:
	test -n $(GOCOVERDIR) && mkdir -p $(GOCOVERDIR)
	$(TEST) $(TEST_FLAGS) $(TEST_TARGET)
	$(GO) tool covdata percent -i=$(GOCOVERDIR) -o $(PROJECT).coverprofile

clean:
	$(RM) -r $(GOCOVERDIR)
