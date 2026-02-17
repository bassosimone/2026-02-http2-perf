// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"github.com/kballard/go-shellquote"
)

func measureGoHTTP2Main(ctx context.Context, args []string) error {
	var (
		http2Flag  = false
		nameFlag   = "ocho"
		methodFlag = ""
	)

	fset := vflag.NewFlagSet("lxs measure gohttp2", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.BoolVar(&http2Flag, '2', "http2", "Force HTTP/2 (default is HTTP/1.1).")
	fset.StringVar(&methodFlag, 'X', "method", "Use the given HTTP `METHOD` (PUT, GET).")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("go build -v ./cmd/gohttp2")
	mustRun("lxc file push cert.pem %s-client/root/", nameFlag)
	mustRun("lxc file push gohttp2 %s-client/root/", nameFlag)

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-client", nameFlag),
		"--",
		"/root/gohttp2",
		"measure",
		"-A",
		serverAddr,
	}
	if http2Flag {
		cmdArgv = append(cmdArgv, "-2")
	}
	if methodFlag != "" {
		cmdArgv = append(cmdArgv, "-X", methodFlag)
	}
	mustRun("%s", shellquote.Join(cmdArgv...))

	return nil
}

func serveGoHTTP2Main(ctx context.Context, args []string) error {
	var (
		nameFlag = "ocho"
	)

	fset := vflag.NewFlagSet("lxs serve gohttp2", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("rm -f cert.pem key.pem")
	mustRun("go build -v ./cmd/gencert")
	mustRun("go build -v ./cmd/gohttp2")

	// Generate certs for the server's IP and push to the server container.
	mustRun("./gencert --ip-addr %s", serverAddr)
	mustRun("lxc file push cert.pem %s-server/root/", nameFlag)
	mustRun("lxc file push key.pem %s-server/root/", nameFlag)
	mustRun("lxc file push gohttp2 %s-server/root/", nameFlag)

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-server", nameFlag),
		"--",
		"/root/gohttp2",
		"serve",
		"-A",
		serverAddr,
	}
	mustRun("%s", shellquote.Join(cmdArgv...))

	return nil
}
