package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/ivan-94/selenium-manager/internal/browserlab/api"
	"github.com/ivan-94/selenium-manager/internal/browserlab/config"
)

const version = "0.1.0"

func main() {
	appSupport, err := config.EnsureAppSupport()
	if err != nil {
		fmt.Fprintf(os.Stderr, "browserlabd: %v\n", err)
		os.Exit(1)
	}

	listenAddr := api.DefaultListenAddr
	if !api.IsLocalListenAddr(listenAddr) {
		fmt.Fprintf(os.Stderr, "browserlabd: refusing non-local listen address %q\n", listenAddr)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "browserlabd: listen %s: %v\n", listenAddr, err)
		os.Exit(1)
	}

	handler := api.NewHandler(api.ServerOptions{
		AppSupport: appSupport,
		ListenAddr: listenAddr,
		Version:    version,
	})

	fmt.Fprintf(os.Stderr, "browserlabd listening on %s\n", listenAddr)
	if err := http.Serve(listener, handler); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "browserlabd: %v\n", err)
		os.Exit(1)
	}
}
