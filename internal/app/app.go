// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package app is Readeck main application.
package app

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/cristalhq/acmd"
	"github.com/phsym/console-slog"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/email"
	"codeberg.org/readeck/readeck/locales"
)

var commands = []acmd.Command{}

type appFlags struct {
	ConfigFile string
}

func (f *appFlags) Flags() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.StringVar(&f.ConfigFile, "config", "config.toml", "configuration file path")

	return fs
}

func fatal(msg string, err error) {
	slog.Error(msg, slog.Any("err", err))
	os.Exit(1)
}

// Run starts the application CLI.
func Run() error {
	return acmd.RunnerOf(commands, acmd.Config{
		AppName:        "readeck",
		AppDescription: "Run Readeck commands",
		Version:        configs.Version(),
	}).Run()
}

// InitApp prepares the app for running the server or the tests.
func InitApp() {
	// Setup logger
	var handler slog.Handler
	if configs.Config.Main.DevMode {
		handler = console.NewHandler(os.Stdout, &console.HandlerOptions{
			Level:      configs.Config.Main.LogLevel,
			Theme:      devLogTheme{},
			TimeFormat: "15:04:05.000",
		})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: configs.Config.Main.LogLevel,
		})
	}

	slog.SetDefault(slog.New(handler))

	// Load locales
	locales.Load()

	// Create required folders
	if err := createFolder(configs.Config.Main.DataDirectory); err != nil {
		fatal("can't create data directory", err)
	}

	// Create content-scripts folder
	if err := createFolder(filepath.Join(configs.Config.Main.DataDirectory, "content-scripts")); err != nil {
		fatal("can't create content-scripts directory", err)
	}
	bookmarks.LoadContentScripts()

	// Database URL
	dsn, err := url.Parse(configs.Config.Database.Source)
	if err != nil {
		fatal("can't read database source value", err)
	}

	// SQLite data path
	if dsn.Scheme == "sqlite3" {
		if err := createFolder(path.Dir(dsn.Opaque)); err != nil {
			fatal("can't create database directory", err)
		}
	}

	// Connect to database
	if err := db.Open(configs.Config.Database.Source); err != nil {
		fatal("can't connect to database", err)
	}

	// Init db schema
	if err := db.Init(); err != nil {
		fatal("can't initialize database", err)
	}

	// Init email sending
	email.InitSender()

	// Set the commissioned flag
	nbUser, err := users.Users.Count()
	if err != nil {
		panic(err)
	}
	configs.Config.Commissioned = nbUser > 0

	// Init the admin user, if necessary
	if err := initAdminUser(); err != nil {
		fatal("can't init initial admin user", err)
	}
}

func appPreRun(flags *appFlags) error {
	if flags.ConfigFile == "" {
		flags.ConfigFile = "config.toml"
	}
	if err := createConfigFile(flags.ConfigFile); err != nil {
		return err
	}

	if err := configs.LoadConfiguration(flags.ConfigFile); err != nil {
		return fmt.Errorf("error loading configuration (%s)", err)
	}

	if err := initConfig(flags.ConfigFile); err != nil {
		return err
	}

	// Enforce debug in dev mode
	if configs.Config.Main.DevMode {
		configs.Config.Main.LogLevel = slog.LevelDebug
	}

	InitApp()

	return nil
}

func appPostRun() {
	if err := db.Close(); err != nil {
		slog.Error("closing database", slog.Any("err", err))
	} else {
		slog.Debug("database is closed")
	}
}

func createConfigFile(filename string) error {
	_, err := os.Stat(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		fd, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		if err = fd.Close(); err != nil {
			return err
		}
	}
	return nil
}

func initConfig(filename string) error {
	// If secret key is empty, we're facing a new configuration file and
	// must write it to a file.
	if configs.Config.Main.SecretKey == "" {
		configs.Config.Main.SecretKey = configs.GenerateKey(64, 96)
		return configs.WriteConfig(filename)
	}

	return nil
}

func createFolder(name string) error {
	stat, err := os.Stat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(name, 0o750); err != nil {
				return err
			}
		} else {
			return err
		}
	} else if !stat.IsDir() {
		return fmt.Errorf("'%s' is not a directory", name)
	}

	return nil
}

func initAdminUser() error {
	// If already commissioned, return early
	if configs.Config.Commissioned {
		return nil
	}

	configAdmin := configs.Config.Admin
	// If no InitialUsername is configured, return early
	if configAdmin.InitialUsername == "" {
		return nil
	}
	// If there's no password hash, return an error
	if configAdmin.InitialPasswordHash == "" {
		return fmt.Errorf("initial admin password hash cannot be empty")
	}

	slog.Info("creating initial admin user", slog.String("initial_username", configAdmin.InitialUsername))

	u := &users.User{
		Username: configAdmin.InitialUsername,
		Password: configAdmin.InitialPasswordHash,
		Email: configAdmin.InitialEmail,
		Group: "admin",
	}

	if err := users.Users.CreateWithHashedPassword(u); err != nil {
		return err
	}

	configs.Config.Commissioned = true

	return nil
}
