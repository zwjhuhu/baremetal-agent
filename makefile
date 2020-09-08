ifndef GOROOT
    $(error GOROOT is not set)
endif

export GO=$(GOROOT)/bin/go
export GOPATH=$(shell pwd)

TARGET_DIR=target
PKG_ZVR_DIR=$(TARGET_DIR)/pkg-zvr
PKG_ZVRBOOT_DIR=$(TARGET_DIR)/pkg-zvrboot
PKG_TAR_DIR=$(TARGET_DIR)/pkg-tar

DEPS=github.com/Sirupsen/logrus github.com/pkg/errors github.com/fatih/structs github.com/prometheus/client_golang/prometheus github.com/bcicen/go-haproxy

baremetal:
	mkdir -p $(TARGET_DIR)
	$(GO) build -o $(TARGET_DIR)/baremetal src/baremetal/baremetal.go

zvrboot:
	mkdir -p $(TARGET_DIR)
	$(GO) build -o $(TARGET_DIR)/zvrboot src/zvr/zvrboot.go

deps:
	$(GO) get $(DEPS)

clean:
	rm -rf target/

tar: baremetal
	rm -rf $(PKG_TAR_DIR)
	mkdir -p $(PKG_TAR_DIR)
	cp -f $(TARGET_DIR)/baremetal $(PKG_TAR_DIR)
	cp -f VERSION $(PKG_TAR_DIR)
	tar czf $(TARGET_DIR)/baremetal.tar.gz -C $(PKG_TAR_DIR) .

