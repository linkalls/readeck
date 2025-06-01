// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"archive/zip"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cristalhq/acmd"

	"codeberg.org/readeck/readeck/internal/portability"
)

func init() {
	commands = append(commands, acmd.Command{
		Name:        "import",
		Description: "Import Readeck data from a file",
		ExecFunc:    runImport,
	})
}

func runImport(_ context.Context, args []string) error {
	var users stringsFlag
	var src string
	var clearData bool

	var flags appFlags
	fs := flags.Flags()
	// nolint: errcheck
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: import [arguments...] FILE")
		fmt.Fprintln(fs.Output(), "  FILE")
		fmt.Fprintln(fs.Output(), "    \tsource file")
		fs.PrintDefaults()
	}
	fs.Var(&users, "user", "username")
	fs.Var(&users, "u", "username (shorthand)")
	fs.BoolVar(&clearData, "clear", false, "clear user data before import")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	src = strings.TrimSpace(fs.Arg(0))

	if src == "" {
		return errors.New("input file is required")
	}

	if clearData && len(users) > 0 {
		return errors.New("cannot use -clear and -user at the same time")
	}

	if clearData {
		fmt.Fprintf( // nolint:errcheck
			fs.Output(),
			"❗ %sAttention!%s This will remove all current users and their data.\n",
			bold, colorReset,
		)
		if !confirmPrompt("Are you sure?", false) {
			return nil
		}
	}

	// Checks and application init
	if err := enforceChecks(&flags); err != nil {
		return fmt.Errorf("Checks failed: %w", err)
	}
	if err := appPreRun(&flags); err != nil {
		return err
	}
	defer appPostRun()

	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()

	loader, err := portability.NewImporter(&zr.Reader, users, clearData)
	if err != nil {
		return err
	}
	loader.SetOutput(os.Stdout)

	fmt.Fprintf(loader.Output(), "%sstarting import%s...\n", colorYellow, colorReset) // nolint:errcheck

	if err = loader.Load(); err != nil {
		return err
	}

	fmt.Fprintf(loader.Output(), "%s%simport done!%s\n", bold, colorGreen, colorReset) // nolint:errcheck

	if clearData {
		return removeOrphanFiles()
	}

	return nil
}
