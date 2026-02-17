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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func serveMain(ctx context.Context, args []string) error {
	var (
		addressFlag = "127.0.0.1"
		portFlag    = "4443"
	)

	fset := vflag.NewFlagSet("gohttp2c serve", vflag.ExitOnError)
	fset.StringVar(&addressFlag, 'A', "address", "Use the given IP `ADDRESS`.")
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&portFlag, 'p', "port", "Use the given TCP `PORT`.")
	runtimex.PanicOnError0(fset.Parse(args))

	mux := http.NewServeMux()
	mux.Handle("GET /{size}", http.HandlerFunc(serveHandleGet))
	mux.Handle("PUT /{size}", http.HandlerFunc(serveHandlePut))

	h2srv := &http2.Server{
		MaxReadFrameSize:             (1 << 24) - 1, // ~16 MiB (protocol max)
		MaxUploadBufferPerConnection: 1 << 30,       // 1 GiB
		MaxUploadBufferPerStream:     1 << 30,       // 1 GiB
	}

	endpoint := net.JoinHostPort(addressFlag, portFlag)
	srv := &http.Server{
		Addr:    endpoint,
		Handler: h2c.NewHandler(mux, h2srv),
	}

	go func() {
		defer srv.Close()
		<-ctx.Done()
	}()

	slog.Info("serving h2c at", slog.String("addr", endpoint))
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
	slog.Info("GET", slog.Int64("count", count), slog.String("proto", req.Proto))
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
	slog.Info("PUT", slog.Int64("expectCount", expectCount), slog.String("proto", req.Proto))
	bodyWrapper := slogging.NewReadCloser(req.Body)
	defer bodyWrapper.Close()
	bodyReader := io.LimitReader(bodyWrapper, expectCount)
	buf := make([]byte, 1<<20) // 1 MiB
	io.CopyBuffer(io.Discard, bodyReader, buf)
	rw.WriteHeader(http.StatusNoContent)
}
