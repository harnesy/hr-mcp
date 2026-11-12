// Command harnsy-hr-mcp serves the hr.harnsy.dev employee catalog over MCP: a read-only, stateless endpoint that any
// agent client can add. It reads the catalog's signed release from disk, verifies it with the catalog public key and
// serves nothing that does not verify.
//
//	harnsy-hr-mcp --root /servers/hr.harnsy.dev --pub <base64 public key> [--kid c1] [--host hr.harnsy.dev] [--listen 127.0.0.1:8795]
//
// It holds no secret and writes nothing. Put it behind a proxy that ends TLS and limits the request rate; see deploy/.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/harnesy/hr-mcp/internal/catalog"
	"github.com/harnesy/hr-mcp/internal/mcpserver"
)

// version is set at build time (-ldflags "-X main.version=…").
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "harnsy-hr-mcp:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("harnsy-hr-mcp", flag.ContinueOnError)
	root := fs.String("root", "", "the catalog's root on disk: the folder that holds current → releases/<seq>")
	pub := fs.String("pub", "", "the catalog public key: base64 of the raw 32-byte Ed25519 key")
	kid := fs.String("kid", "c1", "the key id the catalog's signatures name")
	host := fs.String("host", "hr.harnsy.dev", "the catalog's host: every bundle url of the feed must be on it")
	listen := fs.String("listen", "127.0.0.1:8795", "the address to listen on; keep it on loopback, behind the proxy")
	endpoint := fs.String("endpoint", "", "the public URL of this server, named in install hints (default https://<host>/mcp)")
	every := fs.Duration("every", 30*time.Second, "how often to look for a new release")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *root == "" {
		return errors.New("--root is required")
	}
	key, err := catalog.ParseKey(*kid, *pub)
	if err != nil {
		return fmt.Errorf("--pub, --kid: %w", err)
	}
	if *endpoint == "" {
		*endpoint = "https://" + *host + "/mcp"
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	store := catalog.NewStore(*root, key, *host, time.Now)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go store.Run(ctx, *every, func(err error) {
		if err != nil {
			log.Error("catalog", "error", err.Error())
		} else if rel := store.Release(); rel != nil {
			log.Info("catalog", "seq", rel.Feed.Seq, "employees", len(rel.Employees), "expires", rel.Feed.Expires)
		}
	})

	srv := &http.Server{Addr: *listen, Handler: mcpserver.Handler(store, &mcpserver.Options{Endpoint: *endpoint, Version: version, Logger: log}),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 16 << 10}
	go func() {
		<-ctx.Done()
		down, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(down)
	}()
	log.Info("listening", "addr", *listen, "version", version, "root", *root, "kid", *kid)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
