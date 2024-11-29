// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

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
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/internal/db/filters"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/internal/searchstring"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/locales"
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

type orderExpressionList []exp.OrderedExpression

type createForm struct {
	*forms.Form
	userID    int
	requestID string
	resources []tasks.MultipartResource
}

func newCreateForm(tr forms.Translator, userID int, requestID string) (f *createForm) {
	strConstructor := func(n string) forms.Field {
		return forms.NewTextField(n, forms.Trim)
	}
	strConverter := func(values []forms.Field) interface{} {
		res := make(types.Strings, len(values))
		for i, x := range values {
			res[i] = x.Value().(string)
		}
		return res
	}

	f = &createForm{
		Form: forms.Must(
			forms.NewTextField("url",
				forms.Trim,
				forms.Chain(
					forms.Required,
					forms.IsValidURL(validSchemes...),
				),
			),
			forms.NewTextField("title", forms.Trim),
			forms.NewListField("labels", strConstructor, strConverter),
			forms.NewBooleanField("feature_find_main"),
		),
		userID:    userID,
		requestID: requestID,
	}
	f.SetLocale(tr)
	return
}

// newMultipartResource returns a new instance of multipartResource from
// an io.Reader. The input MUST contain a JSON payload on the first line
// (with the url and headers) and the data on the remaining lines.
func newMultipartResource(r io.Reader) (res tasks.MultipartResource, err error) {
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

func (f *createForm) loadMultipart(r *http.Request) (err error) {
	const maxMemory = 16 << 20 // In MiB

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		return
	}

	// Parse all fields
	for _, field := range f.Fields() {
		for _, v := range r.Form[field.Name()] {
			if err := field.UnmarshalText([]byte(v)); err != nil {
				f.AddErrors(field.Name(), err)
			}
		}
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
			var resource tasks.MultipartResource
			var err error

			if file, err = x.Open(); err != nil {
				return err
			}
			resource, err = newMultipartResource(file)
			if err != nil {
				if f.Get("url").String() != resource.URL {
					// As long as the content is not from the requested URL
					// we can ignore an empty value.
					continue
				}
				return err
			}
			f.resources = append(f.resources, resource)
		}
	}

	return
}

func (f *createForm) createBookmark() (b *bookmarks.Bookmark, err error) {
	if !f.IsBound() {
		return nil, errors.New("form is not bound")
	}

	uri, _ := url.Parse(f.Get("url").String())
	uri.Fragment = ""

	b = &bookmarks.Bookmark{
		UserID:   &f.userID,
		State:    bookmarks.StateLoading,
		URL:      uri.String(),
		Title:    f.Get("title").String(),
		Site:     uri.Hostname(),
		SiteName: uri.Hostname(),
	}

	if !f.Get("labels").IsNil() {
		b.Labels = f.Get("labels").Value().(types.Strings)
		slices.Sort(b.Labels)
		b.Labels = slices.Compact(b.Labels)
	}

	defer func() {
		if err != nil {
			f.AddErrors("", forms.ErrUnexpected)
		}
	}()

	if err = bookmarks.Bookmarks.Create(b); err != nil {
		return
	}

	// Start extraction job
	err = tasks.ExtractPageTask.Run(b.ID, tasks.ExtractParams{
		BookmarkID: b.ID,
		RequestID:  f.requestID,
		Resources:  f.resources,
		FindMain:   f.Get("feature_find_main").IsNil() || f.Get("feature_find_main").Value().(bool),
	})
	return
}

type updateForm struct {
	*forms.Form
}

func newUpdateForm(tr forms.Translator) (f *updateForm) {
	strConstructor := func(n string) forms.Field {
		return forms.NewTextField(n, forms.Trim)
	}
	strConverter := func(values []forms.Field) interface{} {
		res := make(types.Strings, len(values))
		for i, x := range values {
			res[i] = x.Value().(string)
		}
		return res
	}

	f = &updateForm{forms.Must(
		forms.NewTextField("title", forms.Trim),
		forms.NewBooleanField("is_marked"),
		forms.NewBooleanField("is_archived"),
		forms.NewBooleanField("is_deleted"),
		forms.NewListField("labels", strConstructor, strConverter),
		forms.NewListField("add_labels", strConstructor, strConverter),
		forms.NewListField("remove_labels", strConstructor, strConverter),
		forms.NewTextField("_to", forms.Trim),
	)}
	f.SetLocale(tr)
	return
}

func (f *updateForm) update(b *bookmarks.Bookmark) (updated map[string]interface{}, err error) {
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
			// we don't track reading progress yet, so set it based on archival
			if field.Value() == true {
				updated["percent_complete"] = 1
			} else {
				updated["percent_complete"] = 0
			}
		case "is_deleted":
			deleted = new(bool)
			*deleted = field.Value().(bool)
		// labels, add_labels and remove_labels are declared and
		// processed in this order.
		case "labels":
			b.Labels = field.Value().(types.Strings)
			labelsChanged = true
		case "add_labels":
			b.Labels = append(b.Labels, field.Value().(types.Strings)...)
			labelsChanged = true
		case "remove_labels":
			b.Labels = slices.DeleteFunc(b.Labels, func(s string) bool {
				return slices.Contains(field.Value().(types.Strings), s)
			})
			labelsChanged = true
		}
	}

	if labelsChanged {
		slices.SortFunc(b.Labels, db.UnaccentCompare)
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
		df := newDeleteForm(nil)
		df.Get("cancel").Set(!*deleted)
		err = df.trigger(b)
	}

	return
}

type deleteForm struct {
	*forms.Form
}

func newDeleteForm(tr forms.Translator) (f *deleteForm) {
	f = &deleteForm{forms.Must(
		forms.NewBooleanField("cancel"),
		forms.NewTextField("_to", forms.Trim),
	)}
	f.SetLocale(tr)
	return
}

// trigger launch the user deletion or cancel task.
func (f *deleteForm) trigger(b *bookmarks.Bookmark) error {
	if !f.Get("cancel").IsNil() && f.Get("cancel").Value().(bool) {
		return tasks.DeleteBookmarkTask.Cancel(b.ID)
	}

	return tasks.DeleteBookmarkTask.Run(b.ID, b.ID)
}

type labelForm struct {
	*forms.Form
}

func newLabelForm(tr forms.Translator) (f *labelForm) {
	f = &labelForm{
		Form: forms.Must(
			forms.NewTextField("name", forms.Trim, forms.Required),
		),
	}
	f.SetLocale(tr)
	return
}

type labelSearchForm struct {
	*forms.Form
}

func newLabelSearchForm(tr forms.Translator) (f *labelSearchForm) {
	f = &labelSearchForm{forms.Must(
		forms.NewTextField("q", forms.Trim, forms.RequiredOrNil),
	)}
	f.SetLocale(tr)
	return
}

type labelDeleteForm struct {
	*forms.Form
}

func newLabelDeleteForm(tr forms.Translator) (f *labelDeleteForm) {
	f = &labelDeleteForm{
		forms.Must(
			forms.NewBooleanField("cancel"),
		),
	}
	f.SetLocale(tr)
	return
}

func (f *labelDeleteForm) trigger(user *users.User, name string) error {
	id := fmt.Sprintf("%d@%s", user.ID, name)

	if !f.Get("cancel").IsNil() && f.Get("cancel").Value().(bool) {
		return tasks.DeleteLabelTask.Cancel(id)
	}

	return tasks.DeleteLabelTask.Run(id, tasks.LabelDeleteParams{
		UserID: user.ID, Name: name,
	})
}

type filterForm struct {
	*forms.Form
	title        int
	noPagination bool
	sq           searchstring.SearchQuery
}

func newFilterForm(tr forms.Translator) (f *filterForm) {
	availableTypes := [][2]string{
		{"", tr.Gettext("All")},
		{"article", tr.Gettext("Article")},
		{"photo", tr.Gettext("Picture")},
		{"video", tr.Gettext("Video")},
	}

	f = &filterForm{
		Form: forms.Must(
			forms.NewBooleanField("bf"),
			forms.NewTextField("search", forms.Trim),
			forms.NewTextField("title", forms.Trim),
			forms.NewTextField("author", forms.Trim),
			forms.NewTextField("site", forms.Trim),
			forms.NewChoiceField("type", availableTypes, forms.Trim),
			forms.NewBooleanField("is_loaded"),
			forms.NewBooleanField("has_errors"),
			forms.NewBooleanField("has_labels"),
			forms.NewTextField("labels", forms.Trim),
			forms.NewBooleanField("is_marked"),
			forms.NewBooleanField("is_archived"),
			forms.NewTextField("range_start", forms.Trim, validateTimeToken),
			forms.NewTextField("range_end", forms.Trim, validateTimeToken),
			forms.NewDatetimeField("updated_since"),
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
	f.SetLocale(tr)
	return f
}

// newContextFilterForm returns an instance of filterForm. If one already
// exists in the given context, it's reused, otherwise it returns a new one.
func newContextFilterForm(c context.Context, tr forms.Translator) *filterForm {
	ff, ok := c.Value(ctxFiltersKey{}).(*filterForm)
	if !ok {
		ff = newFilterForm(tr)
	}

	return ff
}

func (f *filterForm) Validate() {
	// First, we must build a search string based on
	// the provided free form search and
	// what we might have in the following fields:
	// title, author, site, label
	var err error
	f.sq, err = searchstring.ParseQuery(f.Get("search").String())
	if err != nil {
		f.AddErrors("search", err)
		return
	}

	for _, field := range f.Fields() {
		var fname string
		switch n := field.Name(); n {
		case "title", "author", "site":
			fname = n
		case "labels":
			fname = "label"
		}

		if fname == "" || field.String() == "" {
			continue
		}

		q, err := searchstring.ParseField(field.String(), fname)
		if err != nil {
			f.AddErrors(field.Name(), err)
			continue
		}
		f.sq.Terms = append(f.sq.Terms, q.Terms...)
	}

	// Remove duplicates from the query
	f.sq = f.sq.Dedup()

	// Remove field definition for unallowed fields
	f.sq = f.sq.Unfield("title", "author", "site", "label")

	// Update the specific search fields
	for _, field := range f.Fields() {
		fname := "-"
		switch n := field.Name(); n {
		case "search":
			fname = ""
		case "title", "author", "site":
			fname = n
		case "labels":
			fname = "label"
		}

		if fname == "-" {
			continue
		}
		field.Set(f.sq.ExtractField(fname).RemoveField().String())
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
	var labels searchstring.SearchQuery
	var search searchstring.SearchQuery
	if len(f.sq.Terms) > 0 {
		labels, search = f.sq.PopField("label")
		labels = labels.RemoveField()
	}

	// Label filter
	if len(labels.Terms) > 0 {
		l := make([]exp.BooleanExpression, len(labels.Terms))
		col := goqu.I("b.labels")
		for i, x := range labels.Terms {
			switch {
			case x.Wildcard && x.Exclude:
				l[i] = col.NotLike(x.Value + "%")
			case x.Wildcard:
				l[i] = col.Like(x.Value + "%")
			case x.Exclude:
				l[i] = col.Neq(x.Value)
			default:
				l[i] = col.Eq(x.Value)
			}
		}
		ds = filters.JSONListFilter(ds, l...)
	}

	// Build the search query
	if len(search.Terms) > 0 {
		ds = searchstring.BuildSQL(ds, search, searchConfig[ds.Dialect().Dialect()])
	}

	// Time range
	if f.Get("range_start").String() != "" || f.Get("range_end").String() != "" {
		start, _ := timetoken.New(f.Get("range_start").String())
		if f.Get("range_start").String() == "" {
			// If start is empty, it's an empty time (0001-01-01 00:00:00)
			start.Absolute = &time.Time{}
		}

		end, _ := timetoken.New(f.Get("range_end").String())
		ds = ds.Where(goqu.C("created").Between(
			goqu.Range(start.RelativeTo(nil),
				end.RelativeTo(nil),
			),
		))
	}

	if !f.Get("updated_since").IsNil() {
		ds = ds.Where(goqu.C("updated").Gt(f.Get("updated_since").Value()))
	}

	for _, field := range f.Fields() {
		switch n := field.Name(); n {
		case "is_marked", "is_archived":
			if !field.IsNil() {
				ds = ds.Where(goqu.C(n).Table("b").Eq(goqu.V(field.Value())))
			}
		case "is_loaded":
			if !field.IsNil() {
				ds = ds.Where(db.BooleanExpresion(
					goqu.C("state").Table("b").Neq(bookmarks.StateLoading),
					field.Value().(bool),
				))
			}
		case "has_errors":
			if !field.IsNil() {
				// This one's a bit special. Having errors could be the errors field
				// not being empty or the state being [bookmarks.StateError]
				ds = ds.Where(db.BooleanExpresion(
					goqu.Or(
						goqu.C("state").Table("b").Eq(bookmarks.StateError),
						db.JSONArrayLength(goqu.C("errors").Table("b")).Gt(0),
					),
					field.Value().(bool),
				))
			}
		case "has_labels":
			if !field.IsNil() {
				ds = ds.Where(db.BooleanExpresion(
					db.JSONArrayLength(goqu.C("labels").Table("b")).Gt(0),
					field.Value().(bool),
				))
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

type orderForm struct {
	*forms.Form
	fieldName string
	choices   map[string]exp.Orderable
}

func newOrderForm(fieldName string, choices map[string]exp.Orderable) *orderForm {
	field := forms.NewListField(fieldName, func(n string) forms.Field {
		return forms.NewTextField(n)
	}, func(values []forms.Field) interface{} {
		res := make([]string, len(values))
		for i, x := range values {
			res[i] = x.Value().(string)
		}
		return res
	}, forms.Trim)

	// Compile a list of choices being pairs of "A" and "-A", "B", "-B",
	fieldChoices := make(forms.Choices, len(choices)*2)
	for k := range choices {
		fieldChoices = append(fieldChoices, [2]string{k}, [2]string{"-" + k})
	}

	field.(*forms.ListField).SetChoices(fieldChoices)

	return &orderForm{
		Form:      forms.Must(field),
		fieldName: fieldName,
		choices:   choices,
	}
}

func (f *orderForm) toOrderedExpressions() orderExpressionList {
	if !f.IsBound() || !f.IsValid() {
		return nil
	}
	field := f.Get(f.fieldName)
	value, ok := field.Value().([]string)
	if !ok || len(value) == 0 {
		return nil
	}

	res := orderExpressionList{}
	for _, x := range value {
		identifier := f.choices[strings.TrimPrefix(x, "-")]
		if identifier == nil {
			continue
		}
		if strings.HasPrefix(x, "-") {
			res = append(res, identifier.Desc())
			continue
		}
		res = append(res, identifier.Asc())
	}

	return res
}

func (f *orderForm) value() []string {
	if !f.IsBound() || !f.IsValid() {
		return nil
	}

	if value, ok := f.Get(f.fieldName).Value().([]string); ok {
		return value
	}
	return nil
}

type bookmarkOrderForm struct {
	*orderForm
}

func newBookmarkOrderForm() *bookmarkOrderForm {
	t := goqu.T("b")

	return &bookmarkOrderForm{
		orderForm: newOrderForm("sort", map[string]exp.Orderable{
			"created":   t.Col("created"),
			"domain":    t.Col("domain"),
			"duration":  goqu.Case().When(goqu.L("? > 0", t.Col("duration")), t.Col("duration")).Else(goqu.L("? * 0.3", t.Col("word_count"))),
			"published": goqu.Case().When(t.Col("published").IsNot(nil), t.Col("published")).Else(t.Col("created")),
			"site":      t.Col("site_name"),
			"title":     t.Col("title"),
		}),
	}
}

func (f *bookmarkOrderForm) addToTemplateContext(r *http.Request, tr *locales.Locale, c server.TC) {
	if v := f.value(); len(v) > 0 {
		c["CurrentOrder"] = v[0]
	} else {
		c["CurrentOrder"] = "-created"
	}

	qs := url.Values{}
	for k, v := range r.URL.Query() {
		if k == "sort" {
			continue
		}
		qs[k] = v
	}

	setOption := func(name, label string) [3]string {
		qs["sort"] = []string{name}
		defer delete(qs, "sort")
		return [3]string{name, r.URL.Path + "?" + qs.Encode(), label}
	}

	c["OrderOptions"] = [][3]string{
		setOption("-created", tr.Pgettext("sort", "Added, most recent first")),
		setOption("created", tr.Pgettext("sort", "Added, oldest first")),
		setOption("-published", tr.Pgettext("sort", "Published, most recent first")),
		setOption("published", tr.Pgettext("sort", "Published, oldest first")),
		setOption("title", tr.Pgettext("sort", "Title, A to Z")),
		setOption("-title", tr.Pgettext("sort", "Title, Z to A")),
		setOption("site", tr.Pgettext("sort", "Site Name, A to Z")),
		setOption("-site", tr.Pgettext("sort", "Site Name, Z to A")),
		setOption("duration", tr.Pgettext("sort", "Duration, shortest first")),
		setOption("-duration", tr.Pgettext("sort", "Duration, longest first")),
	}
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

var searchConfig = map[string]*searchstring.BuilderConfig{
	"sqlite3": searchstring.NewBuilderConfig(
		goqu.I("b.id"),
		goqu.I("bookmark_idx.rowid"),
		[][2]string{
			{"", "-catchall"},
			{"title", "title"},
			{"author", "author"},
			{"site", "site"},
			{"label", "label"},
		},
	),
	"postgres": searchstring.NewBuilderConfig(
		goqu.I("b.id"),
		goqu.I("bookmark_search.bookmark_id"),
		[][2]string{
			{"", `bookmark_search.title || bookmark_search.description || bookmark_search."text" || bookmark_search.site || bookmark_search."label"`},
			{"title", "bookmark_search.title"},
			{"author", "bookmark_search.author"},
			{"site", "bookmark_search.site"},
			{"label", "bookmark_search.label"},
		},
	),
}
