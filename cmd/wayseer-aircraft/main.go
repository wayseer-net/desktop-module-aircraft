// Command wayseer-aircraft serves the aircraft module from its own process, for the app to run
// from a signed package; `make sign` builds and signs it.
package main

import (
	"github.com/wayseer-net/desktop-module-aircraft"
	"wayseer.dev/sdk/serve"
)

func main() { serve.Main(aircraft.New()) }
