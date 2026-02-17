// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"os"

	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

func main() {
	serveDisp := vclip.NewDispatcherCommand("lxs serve", vflag.ExitOnError)
	serveDisp.AddCommand("gohttp1", vclip.CommandFunc(serveGoHTTP1Main), "Run gohttp1 service")
	serveDisp.AddCommand("gohttp2", vclip.CommandFunc(serveGoHTTP2Main), "Run gohttp2 service")
	serveDisp.AddCommand("gohttp2c", vclip.CommandFunc(serveGoHTTP2cMain), "Run gohttp2c service")
	serveDisp.AddCommand("ndt7", vclip.CommandFunc(serveNDT7Main), "Run ndt7 service")
	serveDisp.AddCommand("rusthttp2", vclip.CommandFunc(serveRustHTTP2Main), "Run rusthttp2 service")

	measureDisp := vclip.NewDispatcherCommand("lxs measure", vflag.ExitOnError)
	measureDisp.AddCommand("gohttp1", vclip.CommandFunc(measureGoHTTP1Main), "Measure with gohttp1")
	measureDisp.AddCommand("gohttp2", vclip.CommandFunc(measureGoHTTP2Main), "Measure with gohttp2")
	measureDisp.AddCommand("gohttp2c", vclip.CommandFunc(measureGoHTTP2cMain), "Measure with gohttp2c")
	measureDisp.AddCommand("ndt7", vclip.CommandFunc(measureNDT7Main), "Measure with ndt7")
	measureDisp.AddCommand("rusthttp2", vclip.CommandFunc(measureRustHTTP2Main), "Measure with rusthttp2")

	disp := vclip.NewDispatcherCommand("lxs", vflag.ExitOnError)

	disp.AddCommand("create", vclip.CommandFunc(createMain), "Create containers.")
	disp.AddCommand("destroy", vclip.CommandFunc(destroyMain), "Destroy containers.")
	disp.AddCommand("iperf", vclip.CommandFunc(iperfMain), "Run iperf3.")
	disp.AddCommand("measure", measureDisp, "Run measurements.")
	disp.AddCommand("serve", serveDisp, "Run servers.")

	vclip.Main(context.Background(), disp, os.Args[1:])
}
