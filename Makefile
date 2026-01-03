.PHONY: all build build-upstream build-local clean run-upstream run-local test deps

# متغیرها
# dns-local: سرور ایران (کلاینت‌ها به این وصل می‌شوند)
# dns-upstream: سرور خارج (به DNS واقعی وصل می‌شود)
BINARY_LOCAL=dns-local
BINARY_UPSTREAM=dns-upstream
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

build: build-upstream build-local

build-upstream:
	@echo "🔨 ساخت سرور خارج (upstream)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_UPSTREAM) ./cmd/upstream

build-local:
	@echo "🔨 ساخت سرور ایران (local)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_LOCAL) ./cmd/local

# ساخت برای لینوکس
build-linux:
	@echo "🔨 ساخت برای لینوکس..."
	@mkdir -p $(BUILD_DIR)/linux
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/linux/$(BINARY_UPSTREAM) ./cmd/upstream
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/linux/$(BINARY_LOCAL) ./cmd/local

# ساخت برای مک
build-darwin:
	@echo "🔨 ساخت برای macOS..."
	@mkdir -p $(BUILD_DIR)/darwin
	GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/darwin/$(BINARY_UPSTREAM) ./cmd/upstream
	GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/darwin/$(BINARY_LOCAL) ./cmd/local

# ساخت برای ویندوز
build-windows:
	@echo "🔨 ساخت برای ویندوز..."
	@mkdir -p $(BUILD_DIR)/windows
	GOOS=$(GOOS_WINDOWS) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/windows/$(BINARY_UPSTREAM).exe ./cmd/upstream
	GOOS=$(GOOS_WINDOWS) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/windows/$(BINARY_LOCAL).exe ./cmd/local

# ساخت همه پلتفرم‌ها
build-all: build-linux build-darwin build-windows

# اجرای سرور خارج
run-upstream:
	@echo "🚀 اجرای سرور خارج (upstream)..."
	go run ./cmd/upstream -config configs/upstream.yaml

# اجرای سرور ایران
run-local:
	@echo "🚀 اجرای سرور ایران (local)..."
	sudo go run ./cmd/local -config configs/local.yaml

test:
	go test -v ./...

clean:
	@echo "🧹 پاکسازی..."
	rm -rf $(BUILD_DIR)

# تولید salt جدید
generate-salt:
	@go run ./cmd/upstream generate-salt

# نمایش تنظیمات نمونه
generate-config:
	@go run ./cmd/upstream generate-config

# نصب در سیستم
install: build
	@echo "📦 نصب..."
	sudo cp $(BUILD_DIR)/$(BINARY_UPSTREAM) /usr/local/bin/
	sudo cp $(BUILD_DIR)/$(BINARY_LOCAL) /usr/local/bin/

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
	@echo ""
	@echo "  DNS Forwarder با تانل"
	@echo "  ====================="
	@echo ""
	@echo "  dns-local:    سرور ایران - کلاینت‌های شبکه به این وصل می‌شوند"
	@echo "  dns-upstream: سرور خارج - درخواست‌ها را به DNS واقعی می‌فرستد"
	@echo ""
	@echo "  دستورات:"
	@echo "  ---------"
	@echo "  make deps          - دانلود وابستگی‌ها"
	@echo "  make build         - ساخت هر دو سرور"
	@echo "  make build-linux   - ساخت برای لینوکس"
	@echo "  make build-darwin  - ساخت برای macOS"
	@echo "  make build-windows - ساخت برای ویندوز"
	@echo "  make build-all     - ساخت همه پلتفرم‌ها"
	@echo ""
	@echo "  make run-upstream  - اجرای سرور خارج"
	@echo "  make run-local     - اجرای سرور ایران (نیاز به sudo)"
	@echo ""
	@echo "  make test          - اجرای تست‌ها"
	@echo "  make clean         - پاکسازی"
	@echo "  make generate-salt - تولید salt جدید"
	@echo "  make generate-cert - تولید گواهی TLS"
	@echo "  make install       - نصب در سیستم"
