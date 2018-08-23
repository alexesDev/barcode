package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
)

type key int

const (
	requestIDKey key = 0
)

var (
	listenAddr string
	healthy    int32
)

func main() {
	flag.StringVar(&listenAddr, "listen-addr", ":4000", "server listen address")
	flag.Parse()

	logger := log.New(os.Stdout, "http: ", log.LstdFlags)
	logger.Println("Server is starting...")

	router := http.NewServeMux()
	router.Handle("/", index())
	router.Handle("/healthz", healthz())
	router.Handle("/code128/", generateCode128())

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracing(nextRequestID)(logging(logger)(router)),
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		logger.Println("Server is shutting down...")
		atomic.StoreInt32(&healthy, 0)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()

	logger.Println("Server is ready to handle requests at", listenAddr)
	atomic.StoreInt32(&healthy, 1)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	<-done
	logger.Println("Server stopped")
}

func index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.NotFound(w, r)
		fmt.Fprintln(w, "/code128/:width/:height/:content")
	})
}

var validCode128Path = regexp.MustCompile("^/code128/([0-9]+)/([0-9]+)/(.*)$")

func getCode128Params(w http.ResponseWriter, r *http.Request) (int, int, string, error) {
	m := validCode128Path.FindStringSubmatch(r.URL.Path)

	if m == nil {
		http.NotFound(w, r)
		return 0, 0, "", errors.New("Invalid Code128 path")
	}

	width, err := strconv.ParseInt(m[1], 10, 32)

	if err != nil {
		http.NotFound(w, r)
		return 0, 0, "", errors.New("width not valid")
	}

	height, err := strconv.ParseInt(m[2], 10, 32)

	if err != nil {
		http.NotFound(w, r)
		return 0, 0, "", errors.New("height not valid")
	}

	return int(width), int(height), m[3], nil
}

func generateCode128() http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		width, height, content, err := getCode128Params(w, r)

		if err != nil {
			return
		}

		qrCode, _ := code128.EncodeWithoutChecksum(content)
		qrCode, _ = barcode.Scale(qrCode, width, height)

		buffer := new(bytes.Buffer)
		png.Encode(buffer, qrCode)

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))

		if _, err := w.Write(buffer.Bytes()); err != nil {
			log.Println("unable to write image.")
		}
	})
}

func healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Println(requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
