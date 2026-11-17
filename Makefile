GO ?= go
TEST := $(GO) test
TEST_FLAGS ?= -v
TEST_TARGET ?= ./...
GO111MODULE = on
PROJECT := $(shell basename $(PWD))

ifeq ($(origin GOCOVERDIR),undefined)
GOCOVERDIR := $(shell pwd)/_coverage
endif

.PHONY: test coverage integration clean download

download: go.sum

go.sum: go.mod
	$(GO) mod tidy

test: $(GOCOVERDIR)
test: TEST_FLAGS += -json -cover -coverpkg github.com/go-ap/processing -covermode=count -args -test.gocoverdir="$(GOCOVERDIR)"
test: go.sum
	$(TEST) $(TEST_FLAGS) $(TEST_TARGET) | go tool tparse -all -trimpath github.com/go-ap/processing/

integration: $(GOCOVERDIR)
integration: GODOG_FLAGS += -tags.gocoverdir="$(GOCOVERDIR)" --godog.format=pretty --godog.no-colors=false --godog.tags=conformance
integration: TEST_FLAGS += -coverpkg github.com/go-ap/processing -covermode=count
integration: TEST_TARGET = ./integration
integration:
	$(TEST) $(TEST_FLAGS) $(TEST_TARGET) $(GODOG_FLAGS)

$(GOCOVERDIR):
	mkdir -p $(GOCOVERDIR)

coverage: $(GOCOVERDIR)
	$(GO) tool covdata percent -i=$(GOCOVERDIR) -o $(PROJECT).coverprofile
	$(MAKE) clean

clean:
	$(RM) -r $(GOCOVERDIR)
