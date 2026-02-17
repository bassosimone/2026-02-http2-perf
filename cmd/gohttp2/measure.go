// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/bassosimone/2026-02-http2-perf/internal/infinite"
	"github.com/bassosimone/2026-02-http2-perf/internal/slogging"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"golang.org/x/net/http2"
)

func measureMain(ctx context.Context, args []string) error {
	var (
		addressFlag = "127.0.0.1"
		bytesFlag   = int64(1 << 34)
		certFlag    = "cert.pem"
		http2Flag   = false
		methodFlag  = "GET"
		portFlag    = "4443"
	)

	fset := vflag.NewFlagSet("gohttp2 measure", vflag.ExitOnError)
	fset.StringVar(&addressFlag, 'A', "addresss", "Use the given IP `ADDRESS`.")
	fset.Int64Var(&bytesFlag, 'n', "bytes", "Number of bytes to transfer.")
	fset.StringVar(&certFlag, 0, "cert", "Use `FILE` as the CA certificate.")
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.BoolVar(&http2Flag, '2', "http2", "Force HTTP/2 (default is HTTP/1.1).")
	fset.StringVar(&methodFlag, 'X', "method", "Use the given HTTP `METHOD` (PUT, GET).")
	fset.StringVar(&portFlag, 'p', "port", "Use the given TCP `PORT`.")
	runtimex.PanicOnError0(fset.Parse(args))

	runtimex.Assert(methodFlag == "GET" || methodFlag == "PUT")
	runtimex.Assert(certFlag != "")

	// Load the CA certificate to trust the server's self-signed cert.
	caCert := runtimex.LogFatalOnError1(os.ReadFile(certFlag))
	caPool := x509.NewCertPool()
	runtimex.Assert(caPool.AppendCertsFromPEM(caCert))

	tlsConfig := &tls.Config{
		RootCAs: caPool,
	}
	if !http2Flag {
		// Disable HTTP/2 by setting NextProtos to only http/1.1.
		tlsConfig.NextProtos = []string{"http/1.1"}
	}

	transport := &http.Transport{
		TLSClientConfig:   tlsConfig,
		ForceAttemptHTTP2: http2Flag,
	}
	if http2Flag {
		// Tune HTTP/2 for maximum throughput.
		h2transport, err := http2.ConfigureTransports(transport)
		if err == nil {
			h2transport.ReadIdleTimeout = 0
			h2transport.StrictMaxConcurrentStreams = false
		}
	}
	client := &http.Client{Transport: transport}

	URL := &url.URL{
		Scheme: "https",
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

	resp := runtimex.LogFatalOnError1(client.Do(req))
	bodyWrapper := slogging.NewReadCloser(resp.Body)
	defer bodyWrapper.Close()
	slog.Info("response",
		slog.Int("status", resp.StatusCode),
		slog.String("proto", resp.Proto),
		slog.String("alpn", resp.TLS.NegotiatedProtocol),
	)

	buf := make([]byte, 1<<20) // 1 MiB
	_ = runtimex.LogFatalOnError1(io.CopyBuffer(io.Discard, bodyWrapper, buf))

	return nil
}
