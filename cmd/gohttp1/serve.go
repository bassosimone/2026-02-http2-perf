// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"github.com/bassosimone/2026-02-http2-perf/internal/infinite"
	"github.com/bassosimone/2026-02-http2-perf/internal/slogging"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func serveMain(ctx context.Context, args []string) error {
	var (
		addressFlag = "127.0.0.1"
		portFlag    = "8080"
	)

	fset := vflag.NewFlagSet("gohttp1 measure", vflag.ExitOnError)
	fset.StringVar(&addressFlag, 'A', "addresss", "Use the given IP `ADDRESS`.")
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&portFlag, 'p', "port", "Use the given TCP `PORT`.")
	runtimex.PanicOnError0(fset.Parse(args))

	mux := http.NewServeMux()
	mux.Handle("GET /{size}", http.HandlerFunc(serveHandleGet))
	mux.Handle("PUT /{size}", http.HandlerFunc(serveHandlePut))

	endpoint := net.JoinHostPort(addressFlag, portFlag)
	srv := &http.Server{Addr: endpoint, Handler: mux}
	go func() {
		defer srv.Close()
		<-ctx.Done()
	}()

	slog.Info("serving at", slog.String("addr", endpoint))
	err := srv.ListenAndServe()
	slog.Info("interrupted", slog.Any("err", err))

	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	runtimex.LogFatalOnError0(err)
	return nil
}

func serveHandleGet(rw http.ResponseWriter, req *http.Request) {
	count, err := strconv.ParseInt(req.PathValue("size"), 10, 64)
	if err != nil || count < 0 {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	slog.Info("GET", slog.Int64("count", count))
	bodyReader := io.LimitReader(infinite.Reader{}, count)
	rw.Header().Set("Content-Length", strconv.FormatInt(count, 10))
	rw.WriteHeader(http.StatusOK)
	buf := make([]byte, 1<<20) // 1 MiB
	io.CopyBuffer(rw, bodyReader, buf)
}

func serveHandlePut(rw http.ResponseWriter, req *http.Request) {
	expectCount, err := strconv.ParseInt(req.PathValue("size"), 10, 64)
	if err != nil || expectCount < 0 {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	slog.Info("PUT", slog.Int64("expectCount", expectCount))
	bodyWrapper := slogging.NewReadCloser(req.Body)
	defer bodyWrapper.Close()
	bodyReader := io.LimitReader(bodyWrapper, expectCount)
	buf := make([]byte, 1<<20) // 1 MiB
	io.CopyBuffer(io.Discard, bodyReader, buf)
	rw.WriteHeader(http.StatusNoContent)
}
