// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/google/uuid"

	"codeberg.org/readeck/readeck/assets"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/epub"
	"codeberg.org/readeck/readeck/pkg/utils"
)

var uuidURL = uuid.Must(uuid.Parse("6ba7b811-9dad-11d1-80b4-00c04fd430c8"))

func (api *apiRouter) exportBookmarksEPUB(w http.ResponseWriter, r *http.Request, bookmarks ...*Bookmark) {
	if len(bookmarks) == 0 {
		api.srv.Status(w, r, http.StatusNotFound)
		return
	}

	// Define a title
	var title string
	if len(bookmarks) == 1 {
		title = bookmarks[0].Title
	} else if collection, ok := r.Context().Value(ctxCollectionKey{}).(*Collection); ok {
		// In case of a collection, we give the book a title and reverse
		// the items order.
		title = collection.Name
		slices.Reverse(bookmarks)
	} else {
		title = "Readec Bookmarks"
	}

	filename := fmt.Sprintf("%s-%s", bookmarks[0].Created.Format("2006-01-02"), utils.Slug(title))

	id := ""
	for _, x := range bookmarks {
		id += x.UID
	}

	w.Header().Set("Content-Type", "application/epub+zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.epub", filename))

	err := func() (err error) {
		var m *EpubMaker
		if m, err = NewEpubMaker(w, uuid.NewSHA1(uuidURL, []byte(id))); err != nil {
			return
		}
		defer func() {
			if err == nil {
				m.SetTitle(title)
				err = m.WritePackage()
			}
			m.Close()
		}()

		for _, b := range bookmarks {
			if err = m.addBookmark(newBookmarkItem(api.srv, r, b, "")); err != nil {
				return err
			}
		}
		return
	}()
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}
}

// EpubMaker is a wrapper around epub.Writer with extra methods to
// create an epub file from one or many bookmarks.
type EpubMaker struct {
	*epub.Writer
}

// NewEpubMaker creates a new EpubMaker instance.
func NewEpubMaker(w io.Writer, id uuid.UUID) (*EpubMaker, error) {
	m := &EpubMaker{epub.New(w)}
	if err := m.Bootstrap(); err != nil {
		return nil, err
	}

	m.SetID(id.URN())
	m.SetTitle("Readeck ebook")
	m.SetLanguage("en")

	if err := m.addStylesheet(); err != nil {
		return nil, err
	}
	return m, nil
}

// addStylesheet adds the stylesheet to the epub file.
func (m *EpubMaker) addStylesheet() error {
	f, err := assets.StaticFilesFS().Open("epub.css")
	if err != nil {
		return err
	}
	defer f.Close()
	return m.AddFile("stylesheet", "styles/stylesheet.css", "text/css", f)
}

// addBookmark adds a bookmark, with all its resources, to the epub file.
func (m *EpubMaker) addBookmark(b bookmarkItem) (err error) {
	// Open the original container file
	var c *bookmarkContainer
	if c, err = b.OpenContainer(); err != nil {
		return
	}
	defer c.Close()

	// Add all the resource files to the book. They are only images for now.
	for _, x := range c.ListResources() {
		err = func() error {
			fp, err := x.Open()
			if err != nil {
				return err
			}
			defer fp.Close()
			return m.AddImage(
				fmt.Sprintf("res-%s", strings.TrimSuffix(path.Base(x.Name), path.Ext(x.Name))),
				path.Join("Images", path.Base(x.Name)),
				fp,
			)
		}()
		if err != nil {
			return
		}
	}

	// Update the resource paths
	resources := map[string]*bookmarkFile{}
	for k, v := range b.Resources {
		if k == "icon" || k == "image" && (b.Type == "photo" || b.Type == "video") {
			resources[k] = &bookmarkFile{
				Src:    path.Join("Images", fmt.Sprintf("%s-%s%s", k, b.UID, path.Ext(v.Src))),
				Width:  v.Width,
				Height: v.Height,
			}
		}
	}

	// Add all fixed resources (image, icon) to the container
	for k, v := range resources {
		err = func() error {
			fp, err := c.Open(b.Files[k].Name) // original path
			if err != nil {
				return err
			}
			defer fp.Close()

			return m.AddImage(
				fmt.Sprintf("%s-%s", k, b.UID),
				v.Src,
				fp,
			)
		}()
		if err != nil {
			return
		}
	}

	// Swap the item's resources so we can use them in the template
	b.Resources = resources

	// Load the article's content and create the XHTML file
	// that will become a chapter.
	if err = c.LoadArticle(); err != nil {
		if err != os.ErrNotExist {
			return
		}
		err = nil
	}
	if err = c.ReplaceLinks("./_resources", "./Images"); err != nil {
		return
	}
	if err = c.ExtractBody(); err != nil {
		return
	}
	tpl, err := server.GetTemplate("epub/bookmark.jet.html")
	if err != nil {
		return err
	}
	ctx := server.TC{
		"Item":    b,
		"Content": c.GetArticle(),
	}
	buf := new(strings.Builder)
	if err = tpl.Execute(buf, nil, ctx); err != nil {
		return
	}

	// Add the chapter to the book
	m.AddChapter(
		fmt.Sprintf("page-%s", b.UID),
		b.Title,
		fmt.Sprintf("%s.html", b.UID),
		strings.NewReader(buf.String()),
	)

	return
}
