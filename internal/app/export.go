// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
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
		Name:        "export",
		Description: "Export Readeck data to a file",
		ExecFunc:    runExport,
	})
}

func runExport(_ context.Context, args []string) error {
	var users stringsFlag
	var dest string

	var flags appFlags
	fs := flags.Flags()
	// nolint: errcheck
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: import [arguments...] FILE")
		fmt.Fprintln(fs.Output(), "  FILE")
		fmt.Fprintln(fs.Output(), "    \tdestination file")
		fs.PrintDefaults()
	}
	fs.Var(&users, "user", "username")
	fs.Var(&users, "u", "username (shorthand)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	dest = strings.TrimSpace(fs.Arg(0))

	if dest == "" {
		return errors.New("output file is required")
	}

	// Checks and application init
	if err := enforceChecks(&flags); err != nil {
		return fmt.Errorf("Checks failed: %w", err)
	}
	if err := appPreRun(&flags); err != nil {
		return err
	}
	defer appPostRun()

	fd, err := os.Create(dest)
	if err != nil {
		return err
	}

	ex, err := portability.NewExporter(fd, users)
	if err != nil {
		return err
	}
	defer func() {
		if err := ex.Close(); err != nil {
			fatal("error closing the archive", err)
		}
	}()
	ex.SetOutput(os.Stdout)

	fmt.Fprintf(ex.Output(), "%sstarting export%s...\n", colorYellow, colorReset) // nolint:errcheck

	if err = ex.ExportAll(); err != nil {
		return err
	}

	fmt.Fprintf(ex.Output(), "%s%s%s%s created\n", bold, colorGreen, dest, colorReset) // nolint:errcheck
	return nil
}
