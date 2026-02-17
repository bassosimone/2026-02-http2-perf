// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"

	"github.com/bassosimone/2026-02-http2-perf/internal/infinite"
	"github.com/bassosimone/2026-02-http2-perf/internal/slogging"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func measureMain(ctx context.Context, args []string) error {
	var (
		addressFlag = "127.0.0.1"
		bytesFlag   = int64(1 << 34)
		methodFlag  = "GET"
		portFlag    = "8080"
	)

	fset := vflag.NewFlagSet("gohttp1 measure", vflag.ExitOnError)
	fset.StringVar(&addressFlag, 'A', "addresss", "Use the given IP `ADDRESS`.")
	fset.Int64Var(&bytesFlag, 'n', "bytes", "Number of bytes to transfer.")
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&methodFlag, 'X', "method", "Use the given HTTP `METHOD` (PUT, GET).")
	fset.StringVar(&portFlag, 'p', "port", "Use the given TCP `PORT`.")
	runtimex.PanicOnError0(fset.Parse(args))

	runtimex.Assert(methodFlag == "GET" || methodFlag == "PUT")
	URL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(addressFlag, portFlag),
		Path:   fmt.Sprintf("/%d", bytesFlag),
	}
	var body io.Reader = http.NoBody
	if methodFlag == "PUT" {
		runtimex.Assert(bytesFlag >= 1)
		body = io.LimitReader(infinite.Reader{}, bytesFlag)
	}

	req := runtimex.LogFatalOnError1(http.NewRequestWithContext(ctx, methodFlag, URL.String(), body))
	if methodFlag == "PUT" {
		req.ContentLength = bytesFlag
	}
	slog.Info("request", slog.String("method", methodFlag), slog.String("URL", URL.String()))

	resp := runtimex.LogFatalOnError1(http.DefaultClient.Do(req))
	bodyWrapper := slogging.NewReadCloser(resp.Body)
	defer bodyWrapper.Close()
	slog.Info("response", slog.Int("status", resp.StatusCode))

	buf := make([]byte, 1<<20) // 1 MiB
	_ = runtimex.LogFatalOnError1(io.CopyBuffer(io.Discard, bodyWrapper, buf))

	return nil
}
