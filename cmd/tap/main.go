package main

import (
	"os"

	"github.com/tapsh/tap/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
