# DNS Forwarder با تانل رمزنگاری شده

یک DNS Forwarder که درخواست‌های DNS را از طریق تانل WebSocket رمزنگاری شده ارسال می‌کند.

## معماری

```
┌─────────────────┐        ┌────────────────────┐   WSS Tunnel   ┌──────────────┐       ┌──────────────┐
│   کلاینت‌های     │   DNS   │   سرور ایران       │  ─────────────► │  سرور خارج   │ ────► │  8.8.8.8     │
│   شبکه          │ ──────► │   (dns-local)      │  ◄───────────── │ (dns-upstream)│ ◄──── │  1.1.1.1     │
│   PC, موبایل    │         │   Port 53          │                │  Port 8443   │       └──────────────┘
└─────────────────┘        └────────────────────┘                └──────────────┘
```

## اجزا

| نام | محل نصب | توضیح |
|-----|---------|-------|
| `dns-local` | سرور ایران | DNS Server محلی - کلاینت‌های شبکه به این وصل می‌شوند |
| `dns-upstream` | سرور خارج | درخواست‌ها را از تانل دریافت و به DNS واقعی ارسال می‌کند |

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
- دسترسی root برای پورت 53 (در سرور ایران)

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

این مقدار را در هر دو فایل تنظیمات قرار دهید.

### ۲. تنظیم سرور خارج

فایل `configs/upstream.yaml` را ویرایش کنید:

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

### ۳. تنظیم سرور ایران

فایل `configs/local.yaml` را ویرایش کنید:

```yaml
client:
  dns_listen: "0.0.0.0:53"
  server_url: "ws://YOUR_ABROAD_SERVER_IP:8443/dns"
  password: "your-secure-password"
  salt: "your-generated-salt"

cache:
  enabled: true
  ttl: 5m
```

## اجرا

### ۱. ابتدا سرور خارج

```bash
# روی سرور خارج از ایران
./build/dns-upstream -config configs/upstream.yaml
```

### ۲. سپس سرور ایران

```bash
# روی سرور داخل ایران
sudo ./build/dns-local -config configs/local.yaml
```

## استفاده با TLS (توصیه شده)

### ۱. تولید گواهی

```bash
make generate-cert
```

### ۲. تنظیم سرور خارج

```yaml
server:
  listen: ":8443"
  tls_cert: "certs/server.crt"
  tls_key: "certs/server.key"
```

### ۳. تنظیم سرور ایران

```yaml
client:
  server_url: "wss://YOUR_ABROAD_SERVER_IP:8443/dns"
  insecure_skip_tls: true  # برای گواهی خودامضا
```

## تست

```bash
# روی سرور ایران یا هر کلاینت متصل به آن
dig @SERVER_IRAN_IP google.com
nslookup google.com SERVER_IRAN_IP
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
│   ├── local/           # سرور ایران (کلاینت‌ها به این وصل می‌شوند)
│   │   └── main.go
│   └── upstream/        # سرور خارج (به DNS واقعی وصل می‌شود)
│       └── main.go
├── pkg/
│   ├── crypto/          # رمزنگاری AES-GCM
│   │   └── crypto.go
│   └── protocol/        # پروتکل پیام‌رسانی
│       └── message.go
├── configs/
│   ├── local.yaml       # تنظیمات سرور ایران
│   └── upstream.yaml    # تنظیمات سرور خارج
├── Makefile
├── go.mod
└── README.md
```

## نحوه کار

1. کلاینت‌های شبکه (PC، موبایل) درخواست DNS را به سرور ایران ارسال می‌کنند
2. سرور ایران درخواست را رمزنگاری کرده و از طریق تانل WebSocket به سرور خارج ارسال می‌کند
3. سرور خارج درخواست را رمزگشایی کرده و به DNS واقعی (مثل Google DNS) ارسال می‌کند
4. پاسخ از همان مسیر برمی‌گردد

## امنیت

- از رمز عبور قوی استفاده کنید
- از TLS استفاده کنید
- Salt را تغییر دهید
- دسترسی به سرور خارج را محدود کنید

## عیب‌یابی

### خطای "permission denied" برای پورت 53

```bash
sudo ./dns-local -config configs/local.yaml
# یا
sudo setcap cap_net_bind_service=+ep ./dns-local
```

### خطای اتصال به سرور

1. فایروال را بررسی کنید
2. پورت 8443 روی سرور خارج باز باشد
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
