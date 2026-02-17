// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"github.com/kballard/go-shellquote"
)

func measureGoHTTP2cMain(ctx context.Context, args []string) error {
	var (
		nameFlag   = "ocho"
		methodFlag = ""
	)

	fset := vflag.NewFlagSet("lxs measure gohttp2c", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&methodFlag, 'X', "method", "Use the given HTTP `METHOD` (PUT, GET).")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("go build -v ./cmd/gohttp2c")
	mustRun("lxc file push gohttp2c %s-client/root/", nameFlag)

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-client", nameFlag),
		"--",
		"/root/gohttp2c",
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

func serveGoHTTP2cMain(ctx context.Context, args []string) error {
	var (
		nameFlag = "ocho"
	)

	fset := vflag.NewFlagSet("lxs serve gohttp2c", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("go build -v ./cmd/gohttp2c")
	mustRun("lxc file push gohttp2c %s-server/root/", nameFlag)

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-server", nameFlag),
		"--",
		"/root/gohttp2c",
		"serve",
		"-A",
		serverAddr,
	}
	mustRun("%s", shellquote.Join(cmdArgv...))

	return nil
}
