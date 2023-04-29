package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi"
	"golang.org/x/sync/errgroup"
)

// eventsEndpoint returns an http.HandlerFunc that processes an http.Request
// to server sent event.
func eventsEndpoint(events <-chan Event) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")

		flush := func() {}
		if f, ok := w.(http.Flusher); ok {
			flush = f.Flush
		}

		defer func() {
			fmt.Print("events: stream closed\n")
			io.WriteString(w, "event:fin\n\n")
			flush()
		}()

		for {
			select {
			case <-ctx.Done():
				fmt.Print("events: stream cancelled\n")
				return
			case e, more := <-events:
				if !more {
					return
				}
				eventJSON, err := json.Marshal(e)
				if err == nil {
					io.WriteString(w, "data:"+string(eventJSON)+"\n\n")
				} else {
					io.WriteString(w, "error:"+err.Error()+"\n\n")
				}
				flush()
			}
		}
	}
}

func startServer(ctx context.Context, addr, html string, events <-chan Event) {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	})
	r.Get("/events", eventsEndpoint(events))

	srv := &http.Server{
		Addr: addr,
		// ReadTimeout: 75 * time.Second,
		Handler: r,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		defer signal.Stop(c)

		select {
		case <-ctx.Done():
		case <-c:
			cancel()
		}
	}()

	var g errgroup.Group

	g.Go(func() error {
		return srv.ListenAndServe()
	})

	g.Go(func() error {
		<-ctx.Done()
		return srv.Shutdown(ctx)
	})

	if err := g.Wait(); err != nil {
		fmt.Printf("shutdown server: %s", err)
	}
}
