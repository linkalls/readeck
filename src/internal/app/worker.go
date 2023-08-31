// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package app

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/readeck/readeck/internal/bus"
	"github.com/cristalhq/acmd"
	log "github.com/sirupsen/logrus"
)

func init() {
	commands = append(commands, acmd.Command{
		Name:        "worker",
		Description: "Start Readeck jobs worker",
		ExecFunc:    runWorker,
	})
}

func runWorker(_ context.Context, args []string) error {
	var flags appFlags
	if err := flags.Flags().Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Init application
	if err := appPreRun(&flags); err != nil {
		return err
	}
	defer appPostRun()

	// Start jobs worker
	if err := bus.Load(); err != nil {
		return err
	}

	if bus.Protocol() == "memory" {
		return errors.New("cannot start an in memory worker")
	}

	stop := make(chan os.Signal, 2)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	bus.Tasks().Start()
	log.Info("workers started")

	// Shutdown
	<-stop
	log.Info("shutting down...")

	log.Info("stopping workers...")
	bus.Tasks().Stop()
	log.Info("workers stopped")
	return nil
}
