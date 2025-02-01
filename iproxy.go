package main

import (
	"flag"
	"fmt"
	"github.com/armon/go-socks5"
	"github.com/elazarl/goproxy"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Props struct {
	addr      string
	bind      string
	socks     int
	http      int
	discovery int
	verbose   bool
	location  bool
}

func main() {
	props := Props{
		addr: *flag.String("a", "172.20.10.1", "Proxy address to expose to clients"),
		bind: *flag.String("b", "::", "Address to bind to"),
		socks: *flag.Int("s", 1080, "SOCKS5 proxy port"),
		http: *flag.Int("p", 3128, "HTTP proxy port"),
		discovery: *flag.Int("d", 0, "HTTP port for auto proxy configuration discovery"),
		location: *flag.Bool("l", false, "Enable location tracking"),
		verbose: *flag.Bool("v", false, "Enable verbose output"),
	}

	flag.Parse()

	if props.socks != 0 {
		go socksProxy(props)
		go startUDPProxy(fmt.Sprintf("%s:%d", props.addr, props.socks))
	}
	if props.http != 0 {
		go httpProxy(props)
	}
	if props.discovery != 0 {
		go httpAutoDiscover(props)
	}
	if props.location {
		go fetchLocation(props.verbose)
	}

	loop()
}

func socksProxy(props Props) {
	addr := fmt.Sprintf("%s:%d", props.addr, props.socks)
	fmt.Println("Starting SOCKS5 proxy at", addr)
	conf := &socks5.Config{UDPEnabled: true}
	server, err := socks5.New(conf)
	if err != nil {
		log.Fatalf("Failed to start SOCKS5 proxy: %v", err)
	}
	if err := server.ListenAndServe("tcp", addr); err != nil {
		log.Fatalf("SOCKS5 server error: %v", err)
	}
}

func startUDPProxy(addr string) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatalf("Failed to start UDP proxy: %v", err)
	}
	defer conn.Close()
	buf := make([]byte, 2048)
	for {
		n, remoteAddr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("UDP Read error: %v", err)
			continue
		}
		log.Printf("Received UDP packet from %v", remoteAddr)
		_, err = conn.WriteTo([]byte("UDP Proxy Received"), remoteAddr)
		if err != nil {
			log.Printf("UDP Write error: %v", err)
		}
	}
}

func httpProxy(props Props) {
	addr := fmt.Sprintf("%s:%d", props.addr, props.http)
	fmt.Println("Starting HTTP proxy at", addr)
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = props.verbose
	log.Fatal(http.ListenAndServe(addr, proxy))
}

func httpAutoDiscover(props Props) {
	addr := fmt.Sprintf("%s:%d", props.bind, props.discovery)
	fmt.Println("Starting HTTP auto-discovery at", addr)
	http.HandleFunc("/proxy", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(fmt.Sprintf("function FindProxyForURL(url, host) { return 'SOCKS5 %s:%d; DIRECT'; }", props.addr, props.socks)))
	})
	log.Fatal(http.ListenAndServe(addr, nil))
}

func loop() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() { <-c; os.Exit(1) }()
	for {
		time.Sleep(1 * time.Second)
	}
}
