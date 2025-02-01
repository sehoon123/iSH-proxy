package main

import (
	"flag"
	"fmt"
	"github.com/armon/go-socks5"
	"github.com/elazarl/goproxy"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Props struct {
	addr string
	bind string

	socks     int
	http      int
	discovery int

	verbose  bool
	location bool
}

func main() {
	var (
		addr      = flag.String("a", "172.20.10.1", "Proxy address to expose to clients")
		bind      = flag.String("b", "0.0.0.0", "Address to bind to")
		socksPort = flag.Int("s", 0, "SOCKS5 proxy port")
		httpPort  = flag.Int("p", 0, "HTTP proxy port")
		discovery = flag.Int("d", 0, "HTTP port for auto proxy configuration discovery")
		location  = flag.Bool("l", false, "Whether to pool location details")
		verbose   = flag.Bool("v", false, "Enable verbose output")
		help      = flag.Bool("h", false, "Show help")
	)
	flag.Parse()

	props := Props{
		addr:      *addr,
		bind:      *bind,
		socks:     *socksPort,
		http:      *httpPort,
		discovery: *discovery,
		location:  *location,
		verbose:   *verbose,
	}

	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if props.discovery != 0 {
		go httpAutoDiscover(props)
	}
	if props.http != 0 {
		go httpProxy(props)
	}
	if props.socks != 0 {
		go socksProxy(props)
	}
	if props.location {
		go fetchLocation(props.verbose)
	}

	loop()
}

func fetchLocation(verbose bool) {
	fmt.Println("Starting location streaming")
	reader, err := os.Open("/dev/location")
	if err != nil {
		log.Printf("Location data open error: %v", err)
		return
	}
	defer reader.Close()

	p := make([]byte, 256)
	for {
		n, err := reader.Read(p)
		if err != nil {
			if err != io.EOF {
				log.Printf("Location read error: %v", err)
			}
			break
		}
		if verbose && n > 0 {
			fmt.Printf("Location data: %q\n", p[:n])
		}
		time.Sleep(1 * time.Second)
	}
}

func httpAutoDiscover(props Props) {
	addr := fmt.Sprintf("%s:%d", props.bind, props.discovery)
	fmt.Println("Starting http discovery at", addr)
	
	http.HandleFunc("/proxy", func(w http.ResponseWriter, _ *http.Request) {
		if props.verbose {
			fmt.Println("Serving proxy discovery request")
		}
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprintf(w, `function FindProxyForURL(url, host) {
  return '%s%s; DIRECT';}`,
			proxyConfig(props),
		)
	})
	log.Fatal(http.ListenAndServe(addr, nil))
}

func loop() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("\nShutting down...")
	os.Exit(0)
}

func httpProxy(props Props) {
	addr := fmt.Sprint(props.addr, ":", props.http)
	fmt.Println("Starting http proxy at", addr)
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = props.verbose
	log.Fatal(http.ListenAndServe(addr, proxy))
}

func socksProxy(props Props) {
	addr := fmt.Sprintf("%s:%d", props.bind, props.socks)
	fmt.Println("Starting socks5 proxy at", addr)
	
	conf := &socks5.Config{}
	server, err := socks5.New(conf)
	if err != nil {
		log.Fatal("SOCKS5 server creation error:", err)
	}
	
	if err := server.ListenAndServe("tcp", addr); err != nil {
		log.Fatal("SOCKS5 server error:", err)
	}
}

func proxyConfig(props Props) string {
	var config string
	if props.socks != 0 {
		config += fmt.Sprintf("SOCKS5 %s:%d; ", props.addr, props.socks)
	}
	if props.http != 0 {
		config += fmt.Sprintf("HTTP %s:%d; ", props.addr, props.http)
	}
	return config
}