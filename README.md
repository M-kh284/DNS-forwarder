# DNS Forwarder با تانل رمزنگاری شده

یک DNS Forwarder که درخواست‌های DNS را از طریق تانل WebSocket رمزنگاری شده ارسال می‌کند.

## معماری

```
┌─────────────────┐        ┌────────────────────┐   WSS Tunnel   ┌──────────────┐
│   کلاینت‌های     │   DNS   │   سرور ایران       │  ─────────────► │  سرور خارج   │
│   محلی          │ ──────► │   (dns-client)     │  ◄───────────── │  (dns-server)│
│                 │         │   Port 53          │                │  Port 8443   │
└─────────────────┘        └────────────────────┘                └──────────────┘
                                                                        │
                                                                        ▼
                                                                 ┌──────────────┐
                                                                 │  8.8.8.8     │
                                                                 │  1.1.1.1     │
                                                                 └──────────────┘
```

## ویژگی‌ها

- رمزنگاری AES-256-GCM برای تمام ترافیک
- تانل WebSocket (شبیه ترافیک HTTPS)
- پشتیبانی از TLS
- کش DNS محلی
- اتصال مجدد خودکار
- پشتیبانی از چند DNS upstream
- لاگ‌گیری کامل

## نیازمندی‌ها

- Go 1.21 یا بالاتر
- دسترسی root برای پورت 53 (در کلاینت)

## نصب

```bash
# کلون مخزن
git clone https://github.com/your-repo/dns-forwarder.git
cd dns-forwarder

# دانلود وابستگی‌ها
make deps

# ساخت
make build
```

## تنظیمات

### ۱. تولید Salt مشترک

```bash
make generate-salt
```

این مقدار را در هر دو فایل تنظیمات (سرور و کلاینت) قرار دهید.

### ۲. تنظیم سرور (خارج از ایران)

فایل `configs/server.yaml` را ویرایش کنید:

```yaml
server:
  listen: ":8443"
  password: "your-secure-password"
  salt: "your-generated-salt"

dns:
  upstreams:
    - "8.8.8.8:53"
    - "1.1.1.1:53"
```

### ۳. تنظیم کلاینت (داخل ایران)

فایل `configs/client.yaml` را ویرایش کنید:

```yaml
client:
  dns_listen: "127.0.0.1:53"
  server_url: "ws://YOUR_SERVER_IP:8443/dns"
  password: "your-secure-password"
  salt: "your-generated-salt"

cache:
  enabled: true
  ttl: 5m
```

## اجرا

### سرور (خارج)

```bash
# با go run
make run-server

# یا مستقیم
./build/dns-server -config configs/server.yaml
```

### کلاینت (ایران)

```bash
# با go run (نیاز به sudo برای پورت 53)
make run-client

# یا مستقیم
sudo ./build/dns-client -config configs/client.yaml
```

## استفاده با TLS (توصیه شده)

### ۱. تولید گواهی

```bash
make generate-cert
```

### ۲. تنظیم سرور

```yaml
server:
  listen: ":8443"
  tls_cert: "certs/server.crt"
  tls_key: "certs/server.key"
```

### ۳. تنظیم کلاینت

```yaml
client:
  server_url: "wss://YOUR_SERVER_IP:8443/dns"
  insecure_skip_tls: true  # برای گواهی خودامضا
```

## تست

```bash
# بعد از راه‌اندازی کلاینت
dig @127.0.0.1 google.com
nslookup google.com 127.0.0.1
```

## ساخت برای پلتفرم‌های مختلف

```bash
# همه پلتفرم‌ها
make build-all

# فقط لینوکس
make build-linux

# فقط ویندوز
make build-windows
```

## ساختار پروژه

```
.
├── cmd/
│   ├── client/          # کد کلاینت (سرور ایران)
│   │   └── main.go
│   └── server/          # کد سرور (سرور خارج)
│       └── main.go
├── pkg/
│   ├── crypto/          # رمزنگاری AES-GCM
│   │   └── crypto.go
│   └── protocol/        # پروتکل پیام‌رسانی
│       └── message.go
├── configs/
│   ├── client.yaml      # تنظیمات کلاینت
│   └── server.yaml      # تنظیمات سرور
├── Makefile
├── go.mod
└── README.md
```

## امنیت

- از رمز عبور قوی استفاده کنید
- از TLS استفاده کنید
- Salt را تغییر دهید
- دسترسی به سرور را محدود کنید

## عیب‌یابی

### خطای "permission denied" برای پورت 53

```bash
sudo ./dns-client -config configs/client.yaml
# یا
sudo setcap cap_net_bind_service=+ep ./dns-client
```

### خطای اتصال به سرور

1. فایروال را بررسی کنید
2. پورت 8443 باز باشد
3. آدرس سرور درست باشد

### کش DNS سیستم‌عامل

```bash
# لینوکس
sudo systemd-resolve --flush-caches

# macOS
sudo dscacheutil -flushcache
```

## لایسنس

MIT License
