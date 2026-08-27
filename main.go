package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `usage: cred <command> [options]

  cred set  <path> [--name KEY] [--expect-prefix P] [--mode M] [--value-from CMD]
  cred show <path> [--name KEY]
  cred rm   <path> [--name KEY]
  cred version

Run 'cred <command> -h' for the options of one command.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "cred: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
