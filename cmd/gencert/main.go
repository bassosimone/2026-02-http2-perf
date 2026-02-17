// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/bassosimone/pkitest"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

func main() {
	vclip.Main(context.Background(), vclip.CommandFunc(run), os.Args[1:])
}

func run(ctx context.Context, args []string) error {
	var (
		outputDir = "."
		ipAddr    = "127.0.0.1"
	)

	fset := vflag.NewFlagSet("gencert", vflag.ExitOnError)
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.StringVar(&ipAddr, 0, "ip-addr", "Use `ADDR` as an IP SAN.")
	fset.StringVar(&outputDir, 'o', "output-dir", "Write certificates to `DIR`.")
	runtimex.PanicOnError0(fset.Parse(args))

	ip := net.ParseIP(ipAddr)
	if ip == nil {
		log.Fatalf("gencert: invalid IP address: %s", ipAddr)
	}

	config := &pkitest.SelfSignedCertConfig{
		CommonName:   ipAddr,
		DNSNames:     []string{ipAddr},
		IPAddrs:      []net.IP{ip},
		Organization: []string{"ocho"},
	}

	runtimex.LogFatalOnError0(os.MkdirAll(outputDir, 0700))
	pkitest.MustNewSelfSignedCert(config).MustWriteFiles(outputDir)

	log.Printf("gencert: wrote %s", filepath.Join(outputDir, "cert.pem"))
	log.Printf("gencert: wrote %s", filepath.Join(outputDir, "key.pem"))
	return nil
}
