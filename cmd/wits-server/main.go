// Command wits-server serves a wits repository over HTTP: the JSON API in
// pkg/api, and the Angular interface built from wits-ui when told where it is.
//
//	wits-server                                  # the repository here, on 127.0.0.1:7341
//	wits-server --repo ~/wits --ui wits-ui/dist/wits-ui/browser
//
// It listens on this machine only. The repository is health data, and the
// server has no accounts to keep anyone else out, so an address that is not a
// loopback one is refused rather than served.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/TheDonDope/wits/pkg/api"
	"github.com/TheDonDope/wits/pkg/repo"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "🚨 %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("wits-server", flag.ExitOnError)
	dir := fs.String("repo", ".", "the repository to serve, or a directory inside it")
	addr := fs.String("addr", "127.0.0.1:7341", "where to listen; this machine only")
	ui := fs.String("ui", "", "the built interface to serve beside the API")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := loopbackOnly(*addr); err != nil {
		return err
	}
	r, err := repo.Discover(*dir)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(r, *ui).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Printf("wits-server: %s on http://%s\n", r.WorkTree(), *addr)
	return srv.ListenAndServe()
}

// loopbackOnly refuses to listen anywhere but this machine.
func loopbackOnly(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return errors.New("wits-server listens on this machine only: give an address like 127.0.0.1:7341")
	}
	return nil
}
