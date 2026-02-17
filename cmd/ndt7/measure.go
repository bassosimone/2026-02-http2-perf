// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func measureMain(ctx context.Context, args []string) error {
	var (
		addressFlag = "127.0.0.1"
		methodFlag  = "GET"
		portFlag    = "4567"
	)

	fset := vflag.NewFlagSet("ndt7 measure", vflag.ExitOnError)
	fset.StringVar(&addressFlag, 'A', "address", "Use the given IP `ADDRESS`.")
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&methodFlag, 'X', "method", "Use `METHOD` (GET for download, PUT for upload).")
	fset.StringVar(&portFlag, 'p', "port", "Use the given TCP `PORT`.")
	runtimex.PanicOnError0(fset.Parse(args))

	runtimex.Assert(methodFlag == "GET" || methodFlag == "PUT")

	host := net.JoinHostPort(addressFlag, portFlag)

	if methodFlag == "GET" {
		wsURL := fmt.Sprintf("wss://%s/ndt/v7/download", host)
		slog.Info("download", slog.String("url", wsURL))
		conn, err := dial(ctx, wsURL, true)
		runtimex.LogFatalOnError0(err)
		receiver(ctx, conn, "download")
	} else {
		wsURL := fmt.Sprintf("wss://%s/ndt/v7/upload", host)
		slog.Info("upload", slog.String("url", wsURL))
		conn, err := dial(ctx, wsURL, true)
		runtimex.LogFatalOnError0(err)
		sender(ctx, conn, "upload")
	}

	return nil
}
