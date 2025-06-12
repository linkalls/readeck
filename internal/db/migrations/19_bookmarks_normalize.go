// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package migrations

import (
	"io/fs"

	"codeberg.org/readeck/readeck/pkg/utils"
	"github.com/doug-martin/goqu/v9"
)

// M19bookmarkTextNormalization normalize texts in title, description and plain text
// content of every bookmark.
func M19bookmarkTextNormalization(db *goqu.TxDatabase, _ fs.FS) error {
	ds := db.Select(goqu.C("id"), goqu.C("title"), goqu.C("description"), goqu.C("text")).
		From("bookmark")

	bookmarkList := []struct {
		ID          int    `db:"id"`
		Title       string `db:"title"`
		Description string `db:"description"`
		Text        string `db:"text"`
	}{}

	if err := ds.ScanStructs(&bookmarkList); err != nil {
		return err
	}

	for _, x := range bookmarkList {
		newTitle := utils.NormalizeSpaces(x.Title)
		newDescription := utils.NormalizeSpaces(x.Description)
		newText := utils.NormalizeSpaces(x.Text)

		if x.Title == newTitle && x.Description == newDescription && x.Text == newText {
			continue
		}

		_, err := db.Update("bookmark").Prepared(true).
			Set(goqu.Record{
				"title":       newTitle,
				"description": newDescription,
				"text":        newText,
			}).
			Where(goqu.C("id").Eq(x.ID)).
			Executor().Exec()
		if err != nil {
			return err
		}
	}

	return nil
}
