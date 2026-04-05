package main

import (
	"fmt"
	"os"

	"forktool/internal/app"
)

func main() {
	if err := app.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(app.ExitCode(err))
	}
}
