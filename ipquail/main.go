//  Description = ipquail website
//  Author = Theodore Baschak

package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	realIPHeader   = "X-Real-IP"
	healthCheckURL = "http://127.0.0.1:3000/health"
)

type application struct {
	lookupAddr func(string) ([]string, error)
	lookupTXT  func(string) ([]string, error)
}

func newApplication() *application {
	return &application{
		lookupAddr: net.LookupAddr,
		lookupTXT:  net.LookupTXT,
	}
}

func requestIP(w http.ResponseWriter, r *http.Request) (netip.Addr, bool) {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "invalid remote address", http.StatusBadRequest)
		return netip.Addr{}, false
	}
	peerIP := peer.Addr().Unmap()
	if !peerIP.IsLoopback() && !peerIP.IsPrivate() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return netip.Addr{}, false
	}

	ip, err := netip.ParseAddr(r.Header.Get(realIPHeader))
	if err != nil {
		http.Error(w, "invalid client address", http.StatusBadRequest)
		return netip.Addr{}, false
	}

	return ip.Unmap(), true
}

// Reverse returns a reversed slice (array)
func Reverse[T any](original []T) (reversed []T) {
	reversed = make([]T, len(original))
	copy(reversed, original)

	for i := len(reversed)/2 - 1; i >= 0; i-- {
		tmp := len(reversed) - 1 - i
		reversed[i], reversed[tmp] = reversed[tmp], reversed[i]
	}

	return
}

func asnQuery(ip netip.Addr) string {
	if !ip.IsValid() {
		return ""
	}
	if ip.Is4() {
		octets := ip.As4()
		parts := []string{
			strconv.Itoa(int(octets[2])),
			strconv.Itoa(int(octets[1])),
			strconv.Itoa(int(octets[0])),
		}
		return strings.Join(parts, ".") + ".origin.asn.cymru.com"
	}

	octets := ip.As16()
	hexString := hex.EncodeToString(octets[:])
	nibbles := strings.Split(hexString[:16], "") // Aggregate IPv6 lookups at /64.
	return strings.Join(Reverse(nibbles), ".") + ".origin6.asn.cymru.com"
}

// Handles /health
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n")
}

// Handles /ip
func ipHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP, ok := requestIP(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, remoteIP.String()+"\n")
}

// Handles /ptr
func (app *application) ptrHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP, ok := requestIP(w, r)
	if !ok {
		return
	}
	addr, _ := app.lookupAddr(remoteIP.String())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if len(addr) > 0 {
		_, _ = io.WriteString(w, addr[0]+"\n")
	} else {
		_, _ = io.WriteString(w, "none\n")
	}
}

// Handles /asn
func (app *application) asnHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP, ok := requestIP(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	txtrecords, _ := app.lookupTXT(asnQuery(remoteIP))
	if len(txtrecords) > 0 {
		split := strings.Split(txtrecords[0], "|")
		asn := split[0]
		_, _ = io.WriteString(w, asn+"\n")
	} else {
		_, _ = io.WriteString(w, "ERR\n")
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

// Handles /api/ip
func ipApiHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP, ok := requestIP(w, r)
	if !ok {
		return
	}
	writeJSON(w, map[string]string{"ip": remoteIP.String()})
}

// Handles /api/ptr
func (app *application) ptrApiHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP, ok := requestIP(w, r)
	if !ok {
		return
	}
	addr, _ := app.lookupAddr(remoteIP.String())
	if len(addr) > 0 {
		writeJSON(w, map[string]string{"ptr": addr[0]})
	} else {
		writeJSON(w, map[string]string{"ptr": "none"})
	}
}

func (app *application) asnApiHandler(w http.ResponseWriter, r *http.Request) {
	remoteIP, ok := requestIP(w, r)
	if !ok {
		return
	}
	asn := "ERR"
	txtrecords, _ := app.lookupTXT(asnQuery(remoteIP))
	if len(txtrecords) > 0 {
		split := strings.Split(txtrecords[0], "|")
		asn = split[0]
	}

	writeJSON(w, map[string]string{"asn": asn})
}

func newMux(app *application) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /ip", ipHandler)
	mux.HandleFunc("GET /ptr", app.ptrHandler)
	mux.HandleFunc("GET /asn", app.asnHandler)
	mux.HandleFunc("GET /api/ip", ipApiHandler)
	mux.HandleFunc("GET /api/ptr", app.ptrApiHandler)
	mux.HandleFunc("GET /api/asn", app.asnApiHandler)
	return mux
}

func newServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":3000",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func checkHealth(client *http.Client, url string) error {
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status: %s", response.Status)
	}

	return nil
}

func serve() error {
	server := newServer(newMux(newApplication()))

	log.Printf("listening on %s", server.Addr)
	return server.ListenAndServe()
}

func run(args []string, stderr io.Writer, healthCheck, serveServer func() error) int {
	flags := flag.NewFlagSet("ipquail-go", flag.ContinueOnError)
	flags.SetOutput(stderr)
	healthCheckMode := flags.Bool("healthcheck", false, "check whether the backend is healthy")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}

	if *healthCheckMode {
		if err := healthCheck(); err != nil {
			_, _ = fmt.Fprintf(stderr, "health check failed: %v\n", err)
			return 1
		}
		return 0
	}

	if err := serveServer(); err != nil {
		_, _ = fmt.Fprintf(stderr, "server failed: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	healthClient := &http.Client{Timeout: 3 * time.Second}
	os.Exit(run(os.Args[1:], os.Stderr, func() error {
		return checkHealth(healthClient, healthCheckURL)
	}, serve))
}
