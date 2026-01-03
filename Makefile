.PHONY: all build build-server build-client clean run-server run-client test deps

# متغیرها
BINARY_SERVER=dns-server
BINARY_CLIENT=dns-client
BUILD_DIR=build

# سیستم‌عامل هدف
GOOS_LINUX=linux
GOOS_DARWIN=darwin
GOOS_WINDOWS=windows
GOARCH=amd64

all: deps build

deps:
	go mod download
	go mod tidy

build: build-server build-client

build-server:
	@echo "🔨 ساخت سرور..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_SERVER) ./cmd/server

build-client:
	@echo "🔨 ساخت کلاینت..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_CLIENT) ./cmd/client

# ساخت برای لینوکس
build-linux:
	@echo "🔨 ساخت برای لینوکس..."
	@mkdir -p $(BUILD_DIR)/linux
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/linux/$(BINARY_SERVER) ./cmd/server
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/linux/$(BINARY_CLIENT) ./cmd/client

# ساخت برای مک
build-darwin:
	@echo "🔨 ساخت برای macOS..."
	@mkdir -p $(BUILD_DIR)/darwin
	GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/darwin/$(BINARY_SERVER) ./cmd/server
	GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/darwin/$(BINARY_CLIENT) ./cmd/client

# ساخت برای ویندوز
build-windows:
	@echo "🔨 ساخت برای ویندوز..."
	@mkdir -p $(BUILD_DIR)/windows
	GOOS=$(GOOS_WINDOWS) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/windows/$(BINARY_SERVER).exe ./cmd/server
	GOOS=$(GOOS_WINDOWS) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/windows/$(BINARY_CLIENT).exe ./cmd/client

# ساخت همه پلتفرم‌ها
build-all: build-linux build-darwin build-windows

run-server:
	@echo "🚀 اجرای سرور..."
	go run ./cmd/server -config configs/server.yaml

run-client:
	@echo "🚀 اجرای کلاینت..."
	sudo go run ./cmd/client -config configs/client.yaml

test:
	go test -v ./...

clean:
	@echo "🧹 پاکسازی..."
	rm -rf $(BUILD_DIR)

# تولید salt جدید
generate-salt:
	@go run ./cmd/server generate-salt

# نمایش تنظیمات نمونه
generate-config:
	@go run ./cmd/server generate-config

# نصب در سیستم
install: build
	@echo "📦 نصب..."
	sudo cp $(BUILD_DIR)/$(BINARY_SERVER) /usr/local/bin/
	sudo cp $(BUILD_DIR)/$(BINARY_CLIENT) /usr/local/bin/

# تولید گواهی TLS خودامضا
generate-cert:
	@echo "🔐 تولید گواهی TLS..."
	@mkdir -p certs
	openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
		-keyout certs/server.key \
		-out certs/server.crt \
		-subj "/CN=dns-tunnel/O=DNS-Tunnel"
	@echo "✅ گواهی در پوشه certs ذخیره شد"

help:
	@echo "دستورات موجود:"
	@echo "  make deps          - دانلود وابستگی‌ها"
	@echo "  make build         - ساخت سرور و کلاینت"
	@echo "  make build-linux   - ساخت برای لینوکس"
	@echo "  make build-darwin  - ساخت برای macOS"
	@echo "  make build-windows - ساخت برای ویندوز"
	@echo "  make build-all     - ساخت همه پلتفرم‌ها"
	@echo "  make run-server    - اجرای سرور"
	@echo "  make run-client    - اجرای کلاینت (نیاز به sudo)"
	@echo "  make test          - اجرای تست‌ها"
	@echo "  make clean         - پاکسازی"
	@echo "  make generate-salt - تولید salt جدید"
	@echo "  make generate-cert - تولید گواهی TLS"
	@echo "  make install       - نصب در سیستم"
