package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dgrieser/nickpit/workflows"
)

func main() {
	outPath := flag.String("out", "", "Output path. Defaults to stdout.")
	flag.Parse()

	data, err := workflows.ExampleYAML()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if *outPath == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "writing workflow example to stdout: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "writing workflow example to %s: %v\n", *outPath, err)
		os.Exit(1)
	}
}
