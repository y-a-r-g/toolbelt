package toolbelt

import (
	"crypto/tls"
	"golang.org/x/crypto/acme/autocert"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// NewHttpsListener returns a net.Listener that listens on the specified address
// and returns *tls.Conn connections with LetsEncrypt certificates
// for the provided domain or domains.
//
// Use of this function implies acceptance of the LetsEncrypt Terms of
// Service. If domains is not empty, the provided domains are passed
// to HostWhitelist. If domains is empty, the listener will do
// LetsEncrypt challenges for any requested domain, which is not
// recommended.
//
// Certificates are cached in a "golang-autocert" directory under an
// operating system-specific cache or temp directory. This may not
// be suitable for servers spanning multiple machines.
//
// The returned listener uses a *tls.Config that enables HTTP/2, and
// should only be used with servers that support HTTP/2.
//
// The returned Listener also enables TCP keep-alives on the accepted
// connections. The returned *tls.Conn are returned before their TLS
// handshake has completed.
func NewHttpsListener(address string, domains ...string) net.Listener {
	m := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
	}
	if len(domains) > 0 {
		m.HostPolicy = autocert.HostWhitelist(domains...)
	}
	dir := cacheDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("warning: NewHttpsListener not using a cache: %v", err)
	} else {
		m.Cache = autocert.DirCache(dir)
	}

	ln := &listener{
		m:    m,
		conf: m.TLSConfig(),
	}

	if address == "" {
		address = ":https"
	}
	ln.tcpListener, ln.tcpListenErr = net.Listen("tcp", address)
	return ln
}

type listener struct {
	m       *autocert.Manager
	conf    *tls.Config
	address string

	tcpListener  net.Listener
	tcpListenErr error
}

func (ln *listener) Accept() (net.Conn, error) {
	if ln.tcpListenErr != nil {
		return nil, ln.tcpListenErr
	}
	conn, err := ln.tcpListener.Accept()
	if err != nil {
		return nil, err
	}
	tcpConn := conn.(*net.TCPConn)

	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(3 * time.Minute)

	return tls.Server(tcpConn, ln.conf), nil
}

func (ln *listener) Addr() net.Addr {
	if ln.tcpListener != nil {
		return ln.tcpListener.Addr()
	}
	// net.Listen failed. Return something non-nil in case callers
	// call Addr before Accept:
	addr, _ := net.ResolveTCPAddr("tcp", ln.address)
	return addr
}

func (ln *listener) Close() error {
	if ln.tcpListenErr != nil {
		return ln.tcpListenErr
	}
	return ln.tcpListener.Close()
}

func homeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
	}
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "/"
}

func cacheDir() string {
	const base = "golang-autocert"
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir(), "Library", "Caches", base)
	case "windows":
		for _, ev := range []string{"APPDATA", "CSIDL_APPDATA", "TEMP", "TMP"} {
			if v := os.Getenv(ev); v != "" {
				return filepath.Join(v, base)
			}
		}
		// Worst case:
		return filepath.Join(homeDir(), base)
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, base)
	}
	return filepath.Join(homeDir(), ".cache", base)
}
