package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/aaronland/go-http/v3/auth"
	"github.com/aaronland/go-http/v3/server"
)

func main() {

	var authenticator_uri string
	var server_uri string

	flag.StringVar(&authenticator_uri, "authenticator-uri", "null://", "A registered sfomuseum/go-http-auth.Authenticator URI.")
	flag.StringVar(&server_uri, "server-uri", "http://localhost:8080", "...")

	flag.Parse()

	ctx := context.Background()

	authenticator, err := auth.NewAuthenticator(ctx, authenticator_uri)

	if err != nil {
		log.Fatalf("Failed to create new authenticator, %v", err)
	}

	handler := debugHandler(authenticator)
	handler = authenticator.WrapHandler(handler)

	mux := http.NewServeMux()
	mux.Handle("/", handler)

	s, err := server.NewServer(ctx, server_uri)

	if err != nil {
		log.Fatalf("Failed to create server, %v", err)
	}

	log.Printf("Listening for requests at %s\n", s.Address())

	err = s.ListenAndServe(ctx, mux)

	if err != nil {
		log.Fatalf("Failed to serve requests, %v", err)
	}
}

func debugHandler(authenticator auth.Authenticator) http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		acct, err := authenticator.GetAccountForRequest(req)

		if err != nil {

			switch err.(type) {
			case auth.NotLoggedIn:
				slog.Error("Not logged in", "error", err)
				http.Error(rsp, "Forbidden", http.StatusForbidden)
				return
			default:
				slog.Error("Failed to derive account", "error", err)
				http.Error(rsp, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		slog.Info("Authentication successful", "name", acct.Name())

		msg := fmt.Sprintf("Hello, %s (%d)", acct.Name(), acct.Id())
		rsp.Write([]byte(msg))
		return
	}

	return http.HandlerFunc(fn)
}
