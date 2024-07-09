// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package db

import (
	"database/sql"
	"net/url"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres" // dialect
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // std
	log "github.com/sirupsen/logrus"
)

func init() {
	drivers["postgres"] = &pgConnector{}
}

type pgConnector struct {
	dsn    string
	schema string
}

func (c *pgConnector) Name() string {
	return "jackc/pgx"
}

func (c *pgConnector) Dialect() string {
	return "postgres"
}

func (c *pgConnector) Open(dsn *url.URL) (*sql.DB, error) {
	c.dsn = dsn.String()

	config, err := pgconn.ParseConfig(c.dsn)
	if err != nil {
		return nil, err
	}

	// no reason for user to supply multiple schemas in search path, so this should be enough for now for supporting schema
	c.schema = config.RuntimeParams["search_path"]

	if c.schema == "" {
		log.Debug("no schema defined, falling back to public")
		c.schema = "public"
	}

	db, err := sql.Open("pgx", c.dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(2)
	return db, nil
}

func (c *pgConnector) HasTable(name string) (bool, error) {
	ds := Q().Select(goqu.C("tablename")).
		From(goqu.T("pg_tables")).
		Where(
			goqu.C("schemaname").Eq(c.schema),
			goqu.C("tablename").Eq(name),
		)
	var res string

	if _, err := ds.ScanVal(&res); err != nil {
		return false, err
	}

	return res == name, nil
}
