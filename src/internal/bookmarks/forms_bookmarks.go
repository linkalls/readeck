// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/pkg/forms"
	"codeberg.org/readeck/readeck/pkg/timetoken"
)

var validSchemes = []string{"http", "https"}

const (
	filtersTitleUnset = iota
	filtersTitleUnread
	filtersTitleArchived
	filtersTitleFavorites
	filtersTitleArticles
	filtersTitleVideos
	filtersTitlePictures
)

type multipartResource struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Data    []byte            `json:"data"`
}

// newMultipartResource returns a new instance of multipartResource from
// an io.Reader. The input MUST contain a JSON payload on the first line
// (with the url and headers) and the data on the remaining lines.
func newMultipartResource(r io.Reader) (res multipartResource, err error) {
	const bufSize = 256 << 10 // In KiB
	bio := bufio.NewReaderSize(r, bufSize)

	// Read the first line containing the JSON metadata
	var line []byte
	if line, err = bio.ReadBytes('\n'); err != nil {
		return
	}
	if err = json.Unmarshal(line, &res); err != nil {
		return
	}

	if res.URL == "" {
		err = fmt.Errorf("No resource URL")
		return
	}

	// Read the rest (the content)
	res.Data, err = io.ReadAll(bio)
	if err != nil {
		return
	}
	if len(res.Data) == 0 {
		err = fmt.Errorf("No resource content")
		return
	}

	return
}

type createForm struct {
	*forms.Form
	userID    int
	requestID string
	resources []multipartResource
}

func newCreateForm(userID int, requestID string) *createForm {
	return &createForm{
		Form: forms.Must(
			forms.NewTextField("url",
				forms.Trim,
				forms.Required,
				forms.IsValidURL(validSchemes...),
			),
			forms.NewTextField("title", forms.Trim),
		),
		userID:    userID,
		requestID: requestID,
	}
}

func (f *createForm) loadMultipart(r *http.Request) (err error) {
	const maxMemory = 16 << 20 // In MiB

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		return
	}

	if err := f.Get("url").UnmarshalText([]byte(r.FormValue("url"))); err != nil {
		f.AddErrors("url", err)
	}
	if err := f.Get("title").UnmarshalText([]byte(r.FormValue("title"))); err != nil {
		f.AddErrors("title", err)
	}

	forms.Validate(f)
	if !f.IsValid() { // no needs to go further
		return
	}

	// Fetch and store resources
	for k, v := range r.MultipartForm.File {
		if k != "resource" {
			continue
		}
		for _, x := range v {
			var file multipart.File
			var resource multipartResource
			var err error

			if file, err = x.Open(); err != nil {
				return err
			}
			if resource, err = newMultipartResource(file); err != nil {
				return err
			}
			f.resources = append(f.resources, resource)
		}
	}

	return
}

func (f *createForm) createBookmark() (b *Bookmark, err error) {
	if !f.IsBound() {
		return nil, errors.New("form is not bound")
	}

	uri, _ := url.Parse(f.Get("url").String())

	b = &Bookmark{
		UserID:   &f.userID,
		State:    StateLoading,
		URL:      uri.String(),
		Title:    f.Get("title").String(),
		Site:     uri.Hostname(),
		SiteName: uri.Hostname(),
	}

	defer func() {
		if err != nil {
			f.AddErrors("", forms.ErrUnexpected)
		}
	}()

	if err = Bookmarks.Create(b); err != nil {
		return
	}

	// Start extraction job
	err = extractPageTask.Run(b.ID, extractParams{
		BookmarkID: b.ID,
		RequestID:  f.requestID,
		Resources:  f.resources,
	})
	return
}

type updateForm struct {
	*forms.Form
}

func newUpdateForm() *updateForm {
	strConstructor := func(n string) forms.Field {
		return forms.NewTextField(n, forms.Trim)
	}
	strConverter := func(values []forms.Field) interface{} {
		res := make(db.Strings, len(values))
		for i, x := range values {
			res[i] = x.Value().(string)
		}
		return res
	}

	return &updateForm{forms.Must(
		forms.NewTextField("title", forms.Trim),
		forms.NewBooleanField("is_marked"),
		forms.NewBooleanField("is_archived"),
		forms.NewBooleanField("is_deleted"),
		forms.NewListField("labels", strConstructor, strConverter),
		forms.NewListField("add_labels", strConstructor, strConverter),
		forms.NewListField("remove_labels", strConstructor, strConverter),
		forms.NewTextField("_to", forms.Trim),
	)}
}

func (f *updateForm) update(b *Bookmark) (updated map[string]interface{}, err error) {
	updated = map[string]interface{}{}
	var deleted *bool
	labelsChanged := false

	for _, field := range f.Fields() {
		if !field.IsBound() || field.IsNil() {
			continue
		}
		switch n := field.Name(); n {
		case "title":
			if field.Value() != "" {
				b.Title = field.String()
				updated[n] = field.String()
			}
		case "is_marked":
			b.IsMarked = field.Value().(bool)
			updated[n] = field.Value()
		case "is_archived":
			b.IsArchived = field.Value().(bool)
			updated[n] = field.Value()
		case "is_deleted":
			deleted = new(bool)
			*deleted = field.Value().(bool)
		// labels, add_labels and remove_labels are declared and
		// processed in this order.
		case "labels":
			b.Labels = field.Value().(db.Strings)
			labelsChanged = true
		case "add_labels":
			b.Labels = append(b.Labels, field.Value().(db.Strings)...)
			labelsChanged = true
		case "remove_labels":
			b.Labels = slices.DeleteFunc(b.Labels, func(s string) bool {
				return slices.Contains(field.Value().(db.Strings), s)
			})
			labelsChanged = true
		}
	}

	if labelsChanged {
		slices.Sort(b.Labels)
		b.Labels = slices.Compact(b.Labels)
		updated["labels"] = b.Labels
	}

	defer func() {
		updated["id"] = b.UID
		if err != nil {
			f.AddErrors("", forms.ErrUnexpected)
		}
	}()

	if len(updated) > 0 || deleted != nil {
		updated["updated"] = time.Now()
		if err = b.Update(updated); err != nil {
			return
		}

	}

	if deleted != nil {
		updated["is_deleted"] = *deleted
		df := newDeleteForm()
		df.Get("cancel").Set(!*deleted)
		err = df.trigger(b)
	}

	return
}

type deleteForm struct {
	*forms.Form
}

func newDeleteForm() *deleteForm {
	return &deleteForm{forms.Must(
		forms.NewBooleanField("cancel"),
		forms.NewTextField("_to", forms.Trim),
	)}
}

// trigger launch the user deletion or cancel task.
func (f *deleteForm) trigger(b *Bookmark) error {
	if !f.Get("cancel").IsNil() && f.Get("cancel").Value().(bool) {
		return deleteBookmarkTask.Cancel(b.ID)
	}

	return deleteBookmarkTask.Run(b.ID, b.ID)
}

type labelForm struct {
	*forms.Form
}

func newLabelForm() *labelForm {
	return &labelForm{
		Form: forms.Must(
			forms.NewTextField("name", forms.Trim, forms.Required),
		),
	}
}

type labelSearchForm struct {
	*forms.Form
}

func newLabelSearchForm() *labelSearchForm {
	return &labelSearchForm{forms.Must(
		forms.NewTextField("q", forms.Trim, forms.RequiredOrNil),
	)}
}

type labelDeleteForm struct {
	*forms.Form
}

func newLabelDeleteForm() *labelDeleteForm {
	return &labelDeleteForm{
		forms.Must(
			forms.NewBooleanField("cancel"),
		),
	}
}

func (f *labelDeleteForm) trigger(user *users.User, name string) {
	id := fmt.Sprintf("%d@%s", user.ID, name)

	if !f.Get("cancel").IsNil() && f.Get("cancel").Value().(bool) {
		deleteLabelTask.Cancel(id)
		return
	}

	deleteLabelTask.Run(id, labelDeleteParams{
		UserID: user.ID, Name: name,
	})
}

type filterForm struct {
	*forms.Form
	title        int
	noPagination bool
	order        []exp.OrderedExpression
	st           searchString
}

func newFilterForm() *filterForm {
	return &filterForm{
		Form: forms.Must(
			forms.NewBooleanField("bf"),
			forms.NewTextField("search", forms.Trim),
			forms.NewTextField("title", forms.Trim),
			forms.NewTextField("author", forms.Trim),
			forms.NewTextField("site", forms.Trim),
			forms.NewChoiceField("type", append([][2]string{{"", "All"}}, availableTypes...), forms.Trim),
			forms.NewTextField("labels", forms.Trim),
			forms.NewBooleanField("is_marked"),
			forms.NewBooleanField("is_archived"),
			forms.NewTextField("range_start", forms.Trim, validateTimeToken),
			forms.NewTextField("range_end", forms.Trim, validateTimeToken),
			forms.NewListField("id", func(n string) forms.Field {
				return forms.NewTextField(n)
			}, func(values []forms.Field) interface{} {
				res := make([]string, len(values))
				for i, x := range values {
					res[i] = x.Value().(string)
				}
				return res
			}),
		),
		title: filtersTitleUnset,
	}
}

// newContextFilterForm returns an instance of filterForm. If one already
// exists in the given context, it's reused, otherwise it returns a new one.
func newContextFilterForm(c context.Context) *filterForm {
	ff, ok := c.Value(ctxFiltersKey{}).(*filterForm)
	if !ok {
		ff = newFilterForm()
	}

	return ff
}

func (f *filterForm) Validate() {
	// First, we must build a search string based on
	// the provided free form search and
	// what we might have in the following fields:
	// title, author, site, label
	f.st = newSearchString(f.Get("search").String())
	for _, field := range f.Fields() {
		switch n := field.Name(); n {
		case "title", "author", "site":
			f.st.addField(n, field.String())
		case "labels":
			f.st.addField("label", field.String())
		}
	}

	// Remove any duplicate in the final search string
	f.st = f.st.dedup()

	// At this point, any searchString field that is not permitted
	// is considered a quotted value.
	st := searchString{}
	for _, s := range f.st {
		if _, ok := allowedSearchFields[s.Field]; !ok && s.Field != "" {
			st.addField("", fmt.Sprintf(`"%s:%s"`, s.Field, s.Value))
		} else {
			st = append(st, s)
		}
	}
	f.st = st

	// Update the specific search fields
	for _, field := range f.Fields() {
		switch n := field.Name(); n {
		case "search":
			field.Set(f.st.fieldString(""))
		case "title", "author", "site":
			field.Set(f.st.fieldString(n))
		case "labels":
			field.Set(f.st.fieldString("label"))
		}
	}
}

// saveContext returns a context containing this filterForm.
// It can be retrieved using newContextFilterForm().
func (f *filterForm) saveContext(c context.Context) context.Context {
	return context.WithValue(c, ctxFiltersKey{}, f)
}

func (f *filterForm) IsActive() bool {
	if v, ok := f.Get("bf").Value().(bool); ok {
		return v
	}
	return false
}

func (f *filterForm) GetQueryString() string {
	q := url.Values{}
	for _, field := range f.Fields() {
		q.Add(field.Name(), field.String())
	}

	return q.Encode()
}

// setMarked sets the IsMarked property.
func (f *filterForm) setMarked(v bool) {
	f.Get("is_marked").Set(v)
	f.title = filtersTitleFavorites
}

// setArchived sets the IsArchived property.
func (f *filterForm) setArchived(v bool) {
	f.Get("is_archived").Set(v)
	if v {
		f.title = filtersTitleArchived
	} else {
		f.title = filtersTitleUnread
	}
}

func (f *filterForm) setType(v string) {
	f.Get("type").Set(v)
	switch v {
	case "article":
		f.title = filtersTitleArticles
	case "photo":
		f.title = filtersTitlePictures
	case "video":
		f.title = filtersTitleVideos
	}
}

// toSelectDataSet returns an augmented select dataset including all the filter
// clauses.
func (f *filterForm) toSelectDataSet(ds *goqu.SelectDataset) *goqu.SelectDataset {
	// Separate labels from the final search string
	var labels searchString
	var search searchString
	if len(f.st) > 0 {
		labels, search = f.st.popField("label")
	}

	// Label filter
	if len(labels) > 0 {
		l := make([]string, len(labels))
		for i, x := range labels {
			l[i] = x.Value
		}
		ds = Bookmarks.AddLabelFilter(ds, l)
	}

	// Search string
	if len(search) > 0 {
		ds = search.toSelectDataSet(ds)
	}

	// Forced ordering
	if len(f.order) > 0 {
		ds = ds.Order(f.order...)
	}

	// Time range
	if f.Get("range_start").String() != "" {
		start, _ := timetoken.New(f.Get("range_start").String())
		end, _ := timetoken.New(f.Get("range_end").String())
		ds = ds.Where(goqu.C("created").Between(
			goqu.Range(start.RelativeTo(nil),
				end.RelativeTo(nil),
			),
		))
	}

	for _, field := range f.Fields() {
		switch n := field.Name(); n {
		case "is_marked", "is_archived":
			if !field.IsNil() {
				ds = ds.Where(goqu.C(n).Table("b").Eq(goqu.V(field.Value())))
			}
		case "type":
			if field.String() != "" {
				ds = ds.Where(goqu.C("type").Table("b").Eq(field.String()))
			}
		}
	}

	// Filtering by ids. In this case we include all the given IDs and we sort the
	// result according to the IDs order.
	if !f.Get("id").IsNil() {
		ids := f.Get("id").Value().([]string)
		ds = ds.Where(goqu.C("uid").Table("b").In(ids))

		orderging := goqu.Case().Value(goqu.C("uid").Table("b"))
		for i, x := range ids {
			orderging = orderging.When(x, i)
		}
		ds = ds.Order(orderging.Asc())

	}

	return ds
}

func validateTimeToken(f forms.Field) error {
	if f.IsNil() {
		return nil
	}

	if f.String() == "" {
		return nil
	}

	_, err := timetoken.New(f.String())
	if err != nil {
		return fmt.Errorf(`"%s" is not a valid date value`, f.String())
	}

	return nil
}
