package main

import (
	connectors "github.com/francoiscolas/plakar-plugin-b2"

	"os"

	sdk "github.com/PlakarKorp/go-kloset-sdk"
)

func main() {
	sdk.EntrypointStorage(os.Args, connectors.NewStore)
}
