package main

import (
	"os"

	hitkeepcmd "hitkeep/cmd"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "recover" {
		hitkeepcmd.Recover()
		return
	}
	hitkeepcmd.Run()
}
