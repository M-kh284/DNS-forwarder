package main

import (
	"crypto/tls"
	"encoding/hex"
	"flag"
	"log"
	"net"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dns-forwarder/pkg/crypto"
	"github.com/dns-forwarder/pkg/protocol"
	"github.com/gorilla/websocket"
	"github.com/miekg/dns"
	"gopkg.in/yaml.v3"
)

// Config تنظیمات کلاینت
type Config struct {
	Client struct {
		DNSListen       string `yaml:"dns_listen"`
		ServerURL       string `yaml:"server_url"`
		Password        string `yaml:"password"`
		Salt            string `yaml:"salt"`
		InsecureSkipTLS bool   `yaml:"insecure_skip_tls"`
		ReconnectDelay  time.Duration `yaml:"reconnect_delay"`
	} `yaml:"client"`
	Cache struct {
		Enabled bool          `yaml:"enabled"`
		TTL     time.Duration `yaml:"ttl"`
		MaxSize int           `yaml:"max_size"`
	} `yaml:"cache"`
}

// PendingRequest درخواست در انتظار
type PendingRequest struct {
	ResponseChan chan *protocol.Message
	CreatedAt    time.Time
}

// DNSCache کش DNS
type DNSCache struct {
	sync.RWMutex
	entries map[string]*CacheEntry
	maxSize int
}

// CacheEntry ورودی کش
type CacheEntry struct {
	Data      []byte
	ExpiresAt time.Time
}

var (
	configFile     = flag.String("config", "configs/client.yaml", "مسیر فایل تنظیمات")
	config         Config
	encryptor      *crypto.Encryptor
	wsConn         *websocket.Conn
	wsConnMutex    sync.RWMutex
	writeMutex     sync.Mutex
	pendingMutex   sync.RWMutex
	pendingRequests = make(map[uint32]*PendingRequest)
	requestCounter  uint32
	dnsCache       *DNSCache
	connected      int32
)

func main() {
	flag.Parse()

	// خواندن تنظیمات
	if err := loadConfig(*configFile); err != nil {
		log.Fatalf("خطا در خواندن تنظیمات: %v", err)
	}

	// ایجاد رمزنگار
	salt, err := hex.DecodeString(config.Client.Salt)
	if err != nil {
		log.Fatalf("خطا در خواندن salt: %v", err)
	}

	encryptor, err = crypto.NewEncryptor(config.Client.Password, salt)
	if err != nil {
		log.Fatalf("خطا در ایجاد رمزنگار: %v", err)
	}

	// ایجاد کش
	if config.Cache.Enabled {
		dnsCache = &DNSCache{
			entries: make(map[string]*CacheEntry),
			maxSize: config.Cache.MaxSize,
		}
		go cleanupCache()
	}

	// اتصال به سرور
	go connectLoop()

	// راه‌اندازی DNS server محلی
	startDNSServer()
}

func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	// مقادیر پیش‌فرض
	if config.Client.DNSListen == "" {
		config.Client.DNSListen = "127.0.0.1:53"
	}
	if config.Client.ReconnectDelay == 0 {
		config.Client.ReconnectDelay = 5 * time.Second
	}
	if config.Cache.TTL == 0 {
		config.Cache.TTL = 5 * time.Minute
	}
	if config.Cache.MaxSize == 0 {
		config.Cache.MaxSize = 10000
	}

	return nil
}

func connectLoop() {
	for {
		err := connectToServer()
		if err != nil {
			log.Printf("⚠️ خطا در اتصال: %v", err)
		}
		atomic.StoreInt32(&connected, 0)
		log.Printf("🔄 تلاش مجدد برای اتصال در %v...", config.Client.ReconnectDelay)
		time.Sleep(config.Client.ReconnectDelay)
	}
}

func connectToServer() error {
	u, err := url.Parse(config.Client.ServerURL)
	if err != nil {
		return err
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	if config.Client.InsecureSkipTLS {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	log.Printf("🔌 در حال اتصال به %s...", config.Client.ServerURL)

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}

	wsConnMutex.Lock()
	wsConn = conn
	wsConnMutex.Unlock()

	atomic.StoreInt32(&connected, 1)
	log.Printf("✅ متصل به سرور: %s", config.Client.ServerURL)

	// شروع خواندن پیام‌ها
	return readMessages(conn)
}

func readMessages(conn *websocket.Conn) error {
	for {
		messageType, encryptedData, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		// رمزگشایی
		data, err := encryptor.Decrypt(encryptedData)
		if err != nil {
			log.Printf("⚠️ خطا در رمزگشایی: %v", err)
			continue
		}

		// پردازش پیام
		msg, err := protocol.Decode(data)
		if err != nil {
			log.Printf("⚠️ خطا در پردازش پیام: %v", err)
			continue
		}

		switch msg.Type {
		case protocol.TypeDNSResponse:
			handleDNSResponse(msg)
		case protocol.TypeHeartbeatAck:
			// heartbeat تایید شد
		}
	}
}

func handleDNSResponse(msg *protocol.Message) {
	pendingMutex.RLock()
	pending, ok := pendingRequests[msg.RequestID]
	pendingMutex.RUnlock()

	if !ok {
		return
	}

	select {
	case pending.ResponseChan <- msg:
	default:
	}
}

func startDNSServer() {
	server := &dns.Server{
		Addr: config.Client.DNSListen,
		Net:  "udp",
	}

	dns.HandleFunc(".", handleDNSRequest)

	log.Printf("🚀 سرور DNS محلی در حال اجرا روی %s", config.Client.DNSListen)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("خطا در راه‌اندازی DNS server: %v", err)
	}
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	var queryName string
	if len(r.Question) > 0 {
		queryName = r.Question[0].Name
	}

	// بررسی کش
	if config.Cache.Enabled {
		if cached := getCached(queryName); cached != nil {
			response := new(dns.Msg)
			if err := response.Unpack(cached); err == nil {
				response.Id = r.Id
				w.WriteMsg(response)
				log.Printf("📦 کش: %s", queryName)
				return
			}
		}
	}

	// بررسی اتصال
	if atomic.LoadInt32(&connected) == 0 {
		log.Printf("❌ عدم اتصال به سرور برای: %s", queryName)
		response := new(dns.Msg)
		response.SetReply(r)
		response.Rcode = dns.RcodeServerFailure
		w.WriteMsg(response)
		return
	}

	// ایجاد درخواست
	requestID := atomic.AddUint32(&requestCounter, 1)
	dnsData, err := r.Pack()
	if err != nil {
		log.Printf("⚠️ خطا در pack درخواست: %v", err)
		return
	}

	msg := protocol.NewDNSQuery(requestID, dnsData)

	// ثبت درخواست در انتظار
	pending := &PendingRequest{
		ResponseChan: make(chan *protocol.Message, 1),
		CreatedAt:    time.Now(),
	}

	pendingMutex.Lock()
	pendingRequests[requestID] = pending
	pendingMutex.Unlock()

	defer func() {
		pendingMutex.Lock()
		delete(pendingRequests, requestID)
		pendingMutex.Unlock()
	}()

	// ارسال درخواست
	if err := sendMessage(msg); err != nil {
		log.Printf("⚠️ خطا در ارسال درخواست: %v", err)
		response := new(dns.Msg)
		response.SetReply(r)
		response.Rcode = dns.RcodeServerFailure
		w.WriteMsg(response)
		return
	}

	log.Printf("🔍 درخواست: %s (ID: %d)", queryName, requestID)

	// انتظار برای پاسخ
	select {
	case responseMsg := <-pending.ResponseChan:
		response := new(dns.Msg)
		if err := response.Unpack(responseMsg.Payload); err != nil {
			log.Printf("⚠️ خطا در unpack پاسخ: %v", err)
			return
		}

		// ذخیره در کش
		if config.Cache.Enabled && response.Rcode == dns.RcodeSuccess {
			setCache(queryName, responseMsg.Payload)
		}

		// لاگ پاسخ
		if len(response.Answer) > 0 {
			for _, ans := range response.Answer {
				if a, ok := ans.(*dns.A); ok {
					log.Printf("✅ پاسخ: %s -> %s", queryName, a.A.String())
				}
			}
		}

		response.Id = r.Id
		w.WriteMsg(response)

	case <-time.After(10 * time.Second):
		log.Printf("⏱️ تایم‌اوت برای: %s", queryName)
		response := new(dns.Msg)
		response.SetReply(r)
		response.Rcode = dns.RcodeServerFailure
		w.WriteMsg(response)
	}
}

func sendMessage(msg *protocol.Message) error {
	data := msg.Encode()

	encryptedData, err := encryptor.Encrypt(data)
	if err != nil {
		return err
	}

	wsConnMutex.RLock()
	conn := wsConn
	wsConnMutex.RUnlock()

	if conn == nil {
		return nil
	}

	writeMutex.Lock()
	err = conn.WriteMessage(websocket.BinaryMessage, encryptedData)
	writeMutex.Unlock()

	return err
}

func getCached(key string) []byte {
	dnsCache.RLock()
	defer dnsCache.RUnlock()

	entry, ok := dnsCache.entries[key]
	if !ok {
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry.Data
}

func setCache(key string, data []byte) {
	dnsCache.Lock()
	defer dnsCache.Unlock()

	// محدودیت سایز
	if len(dnsCache.entries) >= dnsCache.maxSize {
		// حذف اولین ورودی
		for k := range dnsCache.entries {
			delete(dnsCache.entries, k)
			break
		}
	}

	dnsCache.entries[key] = &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(config.Cache.TTL),
	}
}

func cleanupCache() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		now := time.Now()
		dnsCache.Lock()
		for key, entry := range dnsCache.entries {
			if now.After(entry.ExpiresAt) {
				delete(dnsCache.entries, key)
			}
		}
		dnsCache.Unlock()
	}
}

// پاکسازی درخواست‌های منقضی شده
func init() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			now := time.Now()
			pendingMutex.Lock()
			for id, req := range pendingRequests {
				if now.Sub(req.CreatedAt) > 15*time.Second {
					delete(pendingRequests, id)
				}
			}
			pendingMutex.Unlock()
		}
	}()
}

// resolveServerIP حل IP سرور بدون استفاده از DNS تانل
func resolveServerIP(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}

	host := u.Hostname()
	// اگر قبلاً IP است، برگردان
	if net.ParseIP(host) != nil {
		return serverURL, nil
	}

	// حل با DNS سیستم
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}

	if len(ips) == 0 {
		return serverURL, nil
	}

	// جایگزینی hostname با IP
	port := u.Port()
	if port == "" {
		if u.Scheme == "wss" {
			port = "443"
		} else {
			port = "80"
		}
	}

	return u.Scheme + "://" + ips[0].String() + ":" + port + u.Path, nil
}
