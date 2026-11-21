package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: cc-artex <serve|init|version>")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "serve":
		serve()
	case "init":
		initProject()
	case "version":
		fmt.Println("cc-artex v0.1.0-dev")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func serve() {
	fmt.Println("cc-artex: serve not yet implemented")
	os.Exit(1)
}

func initProject() {
	fmt.Println("cc-artex: init not yet implemented")
	os.Exit(1)
}
