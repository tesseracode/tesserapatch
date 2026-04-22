package main

import (
	"os"

	"github.com/tesseracode/tesserapatch/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
