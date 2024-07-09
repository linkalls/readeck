// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package db

import (
	"database/sql"
	"net/url"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres" // dialect
	_ "github.com/jackc/pgx/v5/stdlib"
)

func init() {
	drivers["postgres"] = &pgConnector{}
}

type pgConnector struct{}

func (c *pgConnector) Name() string {
	return "jackc/pgx"
}

func (c *pgConnector) Dialect() string {
	return "postgres"
}

func (c *pgConnector) Open(dsn *url.URL) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn.String())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(2)
	return db, nil
}

func (c *pgConnector) HasTable(name string) (bool, error) {
	ds := Q().Select(goqu.Func("to_regclass", name))

	var res sql.NullString

	if _, err := ds.ScanVal(&res); err != nil {
		return false, err
	}

	return res.Valid, nil
}
