package main

import (
	"os"

	"github.com/MiniCodeMonkey/tap/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
