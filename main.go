// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package main starts Readeck app subcommands.
package main

import (
	"fmt"
	"os"
	_ "time/tzdata" // load embedded tzdata

	"codeberg.org/readeck/readeck/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}
}
