package main

import (
	connectors "francoiscolas-plakar-b2"

	"os"

	sdk "github.com/PlakarKorp/go-kloset-sdk"
)

func main() {
	sdk.EntrypointStorage(os.Args, connectors.NewStore)
}
