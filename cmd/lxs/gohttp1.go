// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"github.com/kballard/go-shellquote"
)

func measureGoHTTP1Main(ctx context.Context, args []string) error {
	var (
		nameFlag   = "ocho"
		methodFlag = ""
	)

	fset := vflag.NewFlagSet("lxs measure gohttp1", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&methodFlag, 'X', "method", "Use the given HTTP `METHOD` (PUT, GET).")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("go build -v ./cmd/gohttp1")
	mustRun("lxc file push gohttp1 %s-client/root/", nameFlag)

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-client", nameFlag),
		"--",
		"/root/gohttp1",
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

func serveGoHTTP1Main(ctx context.Context, args []string) error {
	var (
		nameFlag = "ocho"
	)

	fset := vflag.NewFlagSet("lxs serve gohttp1", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("go build -v ./cmd/gohttp1")
	mustRun("lxc file push gohttp1 %s-server/root/", nameFlag)

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-server", nameFlag),
		"--",
		"/root/gohttp1",
		"serve",
		"-A",
		serverAddr,
	}
	mustRun("%s", shellquote.Join(cmdArgv...))

	return nil
}
