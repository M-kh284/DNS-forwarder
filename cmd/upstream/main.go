package main

import (
	"crypto/tls"
	"encoding/hex"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dns-forwarder/pkg/crypto"
	"github.com/dns-forwarder/pkg/protocol"
	"github.com/gorilla/websocket"
	"github.com/miekg/dns"
	"gopkg.in/yaml.v3"
)

// Config تنظیمات سرور
type Config struct {
	Server struct {
		Listen   string `yaml:"listen"`
		TLSCert  string `yaml:"tls_cert"`
		TLSKey   string `yaml:"tls_key"`
		Password string `yaml:"password"`
		Salt     string `yaml:"salt"`
	} `yaml:"server"`
	DNS struct {
		Upstreams []string      `yaml:"upstreams"`
		Timeout   time.Duration `yaml:"timeout"`
	} `yaml:"dns"`
}

var (
	configFile = flag.String("config", "configs/upstream.yaml", "مسیر فایل تنظیمات")
	config     Config
	encryptor  *crypto.Encryptor
	dnsClient  *dns.Client
	upgrader   = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

func main() {
	flag.Parse()

	// خواندن تنظیمات
	if err := loadConfig(*configFile); err != nil {
		log.Fatalf("خطا در خواندن تنظیمات: %v", err)
	}

	// ایجاد رمزنگار
	salt, err := hex.DecodeString(config.Server.Salt)
	if err != nil {
		log.Fatalf("خطا در خواندن salt: %v", err)
	}

	encryptor, err = crypto.NewEncryptor(config.Server.Password, salt)
	if err != nil {
		log.Fatalf("خطا در ایجاد رمزنگار: %v", err)
	}

	// ایجاد DNS client
	dnsClient = &dns.Client{
		Net:     "udp",
		Timeout: config.DNS.Timeout,
	}

	// راه‌اندازی HTTP server
	http.HandleFunc("/dns", handleWebSocket)
	http.HandleFunc("/health", handleHealth)

	// بررسی وجود گواهی TLS
	if config.Server.TLSCert != "" && config.Server.TLSKey != "" {
		log.Printf("🚀 سرور DNS Tunnel در حال اجرا روی %s (TLS)", config.Server.Listen)
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		server := &http.Server{
			Addr:      config.Server.Listen,
			TLSConfig: tlsConfig,
		}
		log.Fatal(server.ListenAndServeTLS(config.Server.TLSCert, config.Server.TLSKey))
	} else {
		log.Printf("🚀 سرور DNS Tunnel در حال اجرا روی %s (بدون TLS - فقط برای تست)", config.Server.Listen)
		log.Fatal(http.ListenAndServe(config.Server.Listen, nil))
	}
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
	if config.DNS.Timeout == 0 {
		config.DNS.Timeout = 5 * time.Second
	}
	if len(config.DNS.Upstreams) == 0 {
		config.DNS.Upstreams = []string{"8.8.8.8:53", "1.1.1.1:53"}
	}

	return nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("خطا در ارتقا به WebSocket: %v", err)
		return
	}
	defer conn.Close()

	clientAddr := r.RemoteAddr
	log.Printf("📡 اتصال جدید از: %s", clientAddr)

	var writeMutex sync.Mutex

	// Heartbeat handler
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			writeMutex.Lock()
			err := conn.WriteMessage(websocket.PingMessage, nil)
			writeMutex.Unlock()
			if err != nil {
				return
			}
		}
	}()

	for {
		messageType, encryptedData, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("⚠️ خطا در خواندن پیام: %v", err)
			}
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		// رمزگشایی پیام
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
		case protocol.TypeDNSQuery:
			go handleDNSQuery(conn, &writeMutex, msg, clientAddr)

		case protocol.TypeHeartbeat:
			response := protocol.NewHeartbeatAck()
			sendResponse(conn, &writeMutex, response)
		}
	}

	log.Printf("👋 اتصال بسته شد: %s", clientAddr)
}

func handleDNSQuery(conn *websocket.Conn, mutex *sync.Mutex, msg *protocol.Message, clientAddr string) {
	// parse کردن پکت DNS
	dnsMsg := new(dns.Msg)
	if err := dnsMsg.Unpack(msg.Payload); err != nil {
		log.Printf("⚠️ خطا در parse پکت DNS: %v", err)
		return
	}

	var queryName string
	if len(dnsMsg.Question) > 0 {
		queryName = dnsMsg.Question[0].Name
	}

	log.Printf("🔍 درخواست DNS: %s از %s", queryName, clientAddr)

	// ارسال به upstream DNS
	var response *dns.Msg
	var err error

	for _, upstream := range config.DNS.Upstreams {
		response, _, err = dnsClient.Exchange(dnsMsg, upstream)
		if err == nil {
			break
		}
		log.Printf("⚠️ خطا از upstream %s: %v", upstream, err)
	}

	if err != nil {
		log.Printf("❌ همه upstream ها ناموفق: %v", err)
		// ارسال پاسخ خالی
		response = new(dns.Msg)
		response.SetReply(dnsMsg)
		response.Rcode = dns.RcodeServerFailure
	}

	// pack کردن پاسخ
	responseData, err := response.Pack()
	if err != nil {
		log.Printf("⚠️ خطا در pack پاسخ DNS: %v", err)
		return
	}

	// ایجاد پیام پاسخ
	responseMsg := protocol.NewDNSResponse(msg.RequestID, responseData)
	sendResponse(conn, mutex, responseMsg)

	// لاگ جواب
	if len(response.Answer) > 0 {
		for _, ans := range response.Answer {
			if a, ok := ans.(*dns.A); ok {
				log.Printf("✅ پاسخ: %s -> %s", queryName, a.A.String())
			} else if aaaa, ok := ans.(*dns.AAAA); ok {
				log.Printf("✅ پاسخ: %s -> %s", queryName, aaaa.AAAA.String())
			}
		}
	}
}

func sendResponse(conn *websocket.Conn, mutex *sync.Mutex, msg *protocol.Message) {
	data := msg.Encode()

	encryptedData, err := encryptor.Encrypt(data)
	if err != nil {
		log.Printf("⚠️ خطا در رمزنگاری پاسخ: %v", err)
		return
	}

	mutex.Lock()
	err = conn.WriteMessage(websocket.BinaryMessage, encryptedData)
	mutex.Unlock()

	if err != nil {
		log.Printf("⚠️ خطا در ارسال پاسخ: %v", err)
	}
}

// generateConfig تولید فایل تنظیمات نمونه
func generateConfig() {
	salt, _ := crypto.GenerateSalt()

	cfg := Config{}
	cfg.Server.Listen = ":8443"
	cfg.Server.TLSCert = ""
	cfg.Server.TLSKey = ""
	cfg.Server.Password = "change-this-password"
	cfg.Server.Salt = hex.EncodeToString(salt)
	cfg.DNS.Upstreams = []string{"8.8.8.8:53", "1.1.1.1:53"}
	cfg.DNS.Timeout = 5 * time.Second

	data, _ := yaml.Marshal(cfg)
	log.Printf("نمونه تنظیمات:\n%s", string(data))
}

func init() {
	// تنظیم لاگ
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// بررسی آرگومان برای تولید salt
	if len(os.Args) > 1 && os.Args[1] == "generate-salt" {
		salt, _ := crypto.GenerateSalt()
		log.Printf("Salt جدید: %s", hex.EncodeToString(salt))
		os.Exit(0)
	}

	if len(os.Args) > 1 && os.Args[1] == "generate-config" {
		generateConfig()
		os.Exit(0)
	}
}

// isLocalIP بررسی اینکه IP محلی است یا نه
func isLocalIP(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}
