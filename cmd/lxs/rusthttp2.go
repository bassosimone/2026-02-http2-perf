// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"github.com/kballard/go-shellquote"
)

func measureRustHTTP2Main(ctx context.Context, args []string) error {
	var (
		nameFlag   = "ocho"
		methodFlag = ""
		noTLSFlag  = false
	)

	fset := vflag.NewFlagSet("lxs measure rusthttp2", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&methodFlag, 'X', "method", "Use the given HTTP `METHOD` (PUT, GET).")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	fset.BoolVar(&noTLSFlag, 0, "no-tls", "Use h2c (HTTP/2 over cleartext).")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("cargo build --release --target x86_64-unknown-linux-musl --manifest-path cmd/rusthttp2/Cargo.toml")
	mustRun("cp cmd/rusthttp2/target/x86_64-unknown-linux-musl/release/rusthttp2 .")
	mustRun("lxc file push rusthttp2 %s-client/root/", nameFlag)
	if !noTLSFlag {
		mustRun("lxc file push testdata/cert.pem %s-client/root/", nameFlag)
	}

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-client", nameFlag),
		"--",
		"/root/rusthttp2",
		"measure",
		"-A",
		serverAddr,
	}
	if noTLSFlag {
		cmdArgv = append(cmdArgv, "--no-tls")
	}
	if methodFlag != "" {
		cmdArgv = append(cmdArgv, "-X", methodFlag)
	}
	mustRun("%s", shellquote.Join(cmdArgv...))

	return nil
}

func serveRustHTTP2Main(ctx context.Context, args []string) error {
	var (
		nameFlag  = "ocho"
		noTLSFlag = false
	)

	fset := vflag.NewFlagSet("lxs serve rusthttp2", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&nameFlag, 'n', "name", "Use `NAME` to name LXC resources.")
	fset.BoolVar(&noTLSFlag, 0, "no-tls", "Use h2c (HTTP/2 over cleartext).")
	runtimex.PanicOnError0(fset.Parse(args))

	mustRun("cargo build --release --target x86_64-unknown-linux-musl --manifest-path cmd/rusthttp2/Cargo.toml")
	mustRun("cp cmd/rusthttp2/target/x86_64-unknown-linux-musl/release/rusthttp2 .")

	if !noTLSFlag {
		mustRun("go build -v ./cmd/gencert")
		mustRun("./gencert --ip-addr %s", serverAddr)
		mustRun("lxc file push testdata/cert.pem %s-server/root/", nameFlag)
		mustRun("lxc file push testdata/key.pem %s-server/root/", nameFlag)
	}
	mustRun("lxc file push rusthttp2 %s-server/root/", nameFlag)

	cmdArgv := []string{
		"lxc",
		"exec",
		fmt.Sprintf("%s-server", nameFlag),
		"--",
		"/root/rusthttp2",
		"serve",
		"-A",
		serverAddr,
	}
	if noTLSFlag {
		cmdArgv = append(cmdArgv, "--no-tls")
	}
	mustRun("%s", shellquote.Join(cmdArgv...))

	return nil
}
