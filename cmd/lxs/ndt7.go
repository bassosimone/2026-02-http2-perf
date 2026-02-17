// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"github.com/kballard/go-shellquote"
)

func measureNDT7Main(ctx context.Context, args []string) error {
	var (
		nameFlag   = "ocho"
		methodFlag = ""
	)

	fset := vflag.NewFlagSet("lxs measure ndt7", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&methodFlag, 'X', "method", "Use the given HTTP `METHOD` (GET for download, PUT for upload).")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("go build -v ./cmd/ndt7")
	mustRun("lxc file push ndt7 %s-client/root/", nameFlag)

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-client", nameFlag),
		"--",
		"/root/ndt7",
		"measure",
		"-A",
		serverAddr,
	}
	if methodFlag != "" {
		cmdArgv = append(cmdArgv, "-X", methodFlag)
	}
	mustRun("%s", shellquote.Join(cmdArgv...))

	return nil
}

func serveNDT7Main(ctx context.Context, args []string) error {
	var (
		nameFlag = "ocho"
	)

	fset := vflag.NewFlagSet("lxs serve ndt7", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("go build -v ./cmd/gencert")
	mustRun("go build -v ./cmd/ndt7")

	mustRun("./gencert --ip-addr %s", serverAddr)
	mustRun("lxc file push testdata/cert.pem %s-server/root/", nameFlag)
	mustRun("lxc file push testdata/key.pem %s-server/root/", nameFlag)
	mustRun("lxc file push ndt7 %s-server/root/", nameFlag)

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-server", nameFlag),
		"--",
		"/root/ndt7",
		"serve",
		"-A",
		serverAddr,
	}
	mustRun("%s", shellquote.Join(cmdArgv...))

	return nil
}
