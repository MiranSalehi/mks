// Package translations lets a manager change the panel's own texts and add languages
// (MKSine's translations editor and its "Add language" action, which edit language files;
// here both are rows in the database). The UI receives them in meta.extra.translations
// and meta.extra.locales. A new language is translated over a base language (fa or en):
// texts not translated yet show in the base, and the server answers in the base too.
// Its frontend half is @mks/mks/translations.
//
//	p.Use(&translations.Plugin{Orm: facades.Orm()})
//
// The tables come from migrations: translations.CreateTable and CreateLocalesTable.
package translations

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"

	mks "github.com/MiranSalehi/mks"
)

// Table holds the edited texts.
const Table = "panel_translations"

// Manage is the permission to change texts.
const Manage = "translations.manage"

// Locales are the languages the panel UI is built with; others are added in the editor.
var Locales = []string{"fa", "en"}

// LocalesTable holds the added languages.
const LocalesTable = "panel_locales"

// CreateLocalesTable creates the added-languages table; call it from a migration.
func CreateLocalesTable(s schema.Schema) error {
	return s.Create(LocalesTable, func(t schema.Blueprint) {
		t.ID()
		t.String("code", 12)
		t.String("name", 60)
		t.String("direction", 3).Default("ltr")
		t.String("base", 12).Default("en")
		t.String("tag", 35).Default("")
		t.Unique("code")
	})
}

// Locale is a language of the panel: built in, or added by a manager.
type Locale struct {
	ID        uint   `gorm:"primaryKey" json:"-"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	// Base is the built-in language whose texts fill what is not translated.
	Base string `json:"base"`
	// Tag is the BCP 47 tag for numbers and dates (e.g. "ckb-IQ"); empty: the base's.
	Tag     string `json:"tag"`
	BuiltIn bool   `gorm:"-" json:"built_in"`
}

func (Locale) TableName() string { return LocalesTable }

var builtIn = []Locale{
	{Code: "fa", Name: "Persian", Direction: "rtl", Base: "fa", Tag: "fa-IR", BuiltIn: true},
	{Code: "en", Name: "English", Direction: "ltr", Base: "en", Tag: "en-US", BuiltIn: true},
}

var (
	codeRE = regexp.MustCompile(`^[a-z]{2,3}(_[A-Za-z0-9]{1,8})?$`)
	tagRE  = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8}){0,3}$`)
)

// CreateTable creates the table; call it from a migration.
func CreateTable(s schema.Schema) error {
	return s.Create(Table, func(t schema.Blueprint) {
		t.ID()
		t.String("locale", 12)
		t.String("name", 120)
		t.Text("value")
		t.Unique("locale", "name")
	})
}

type row struct {
	ID     uint   `gorm:"primaryKey"`
	Locale string `gorm:"column:locale"`
	Name   string `gorm:"column:name"`
	Value  string `gorm:"column:value"`
}

func (row) TableName() string { return Table }

var keyRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.]{0,119}$`)

// Plugin is the translations editor. Orm is required.
type Plugin struct {
	Orm orm.Orm

	mu      sync.RWMutex
	gen     int // bumped on every save; a load that raced a save does not cache
	cached  map[string]map[string]string
	locales []Locale
}

func (*Plugin) Name() string { return "translations" }

func (pl *Plugin) Register(p *mks.Panel) {
	if pl.Orm == nil {
		panic("translations: Orm is required")
	}
	p.Permission(Manage)
	p.MetaValue("translations", func(*mks.HookContext) any {
		m, _, err := pl.load()
		if err != nil {
			return nil
		}
		return m
	})
	p.MetaValue("locales", func(*mks.HookContext) any {
		_, locales, err := pl.load()
		if err != nil {
			return nil
		}
		return locales
	})
	p.Pages(mks.Page{
		Name:      "translations",
		Authorize: func(ctx *mks.HookContext) bool { return p.Allowed(ctx, Manage) },
		Data: func(*mks.WidgetContext) (any, error) {
			m, locales, err := pl.load()
			return map[string]any{"overrides": m, "locales": locales}, err
		},
		Handle: func(ctx *mks.PageContext) (*mks.ActionResult, error) {
			var result *mks.ActionResult
			// In a transaction of its own, so the cache is dropped only after the commit (a
			// read between the two would otherwise cache the old texts again).
			err := pl.Orm.Transaction(func(tx orm.Query) error {
				var err error
				switch action, _ := ctx.Data["action"].(string); action {
				case "add_locale":
					result, err = pl.addLocale(tx, ctx.Data)
				case "remove_locale":
					result, err = pl.removeLocale(tx, ctx.Data)
				default:
					result, err = pl.saveTexts(tx, ctx.Data)
				}
				return err
			})
			if err != nil {
				return nil, err
			}
			pl.mu.Lock()
			pl.cached, pl.locales, pl.gen = nil, nil, pl.gen+1
			pl.mu.Unlock()
			return result, nil
		},
	})
}

// load gives the edited texts by locale and every language, built-in ones first.
func (pl *Plugin) load() (map[string]map[string]string, []Locale, error) {
	pl.mu.RLock()
	m, locales, gen := pl.cached, pl.locales, pl.gen
	pl.mu.RUnlock()
	if m != nil {
		return m, locales, nil
	}
	var rows []row
	if err := pl.Orm.Query().Model(&row{}).Find(&rows); err != nil {
		return nil, nil, err
	}
	var added []Locale
	if err := pl.Orm.Query().Model(&Locale{}).Order("code").Find(&added); err != nil {
		return nil, nil, err
	}
	m = map[string]map[string]string{}
	for _, r := range rows {
		if m[r.Locale] == nil {
			m[r.Locale] = map[string]string{}
		}
		m[r.Locale][r.Name] = r.Value
	}
	locales = append(slices.Clone(builtIn), added...)
	pl.mu.Lock()
	if pl.gen == gen {
		pl.cached, pl.locales = m, locales
	}
	pl.mu.Unlock()
	return m, locales, nil
}

func (pl *Plugin) known(q orm.Query, code string) (bool, error) {
	if slices.Contains(Locales, code) {
		return true, nil
	}
	n, err := q.Model(&Locale{}).Where("code = ?", code).Count()
	return n > 0, err
}

func (pl *Plugin) saveTexts(q orm.Query, data map[string]any) (*mks.ActionResult, error) {
	locale, _ := data["locale"].(string)
	values, _ := data["values"].(map[string]any)
	ok, err := pl.known(q, locale)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, mks.ValidationErrors{"locale": "Invalid language."}
	}
	if err := pl.save(q, locale, values); err != nil {
		return nil, err
	}
	return mks.Success("Texts saved; refresh the page to see the change."), nil
}

// addLocale adds a language: code ("ckb", "ku", "ar_IQ"), name, direction, base and tag.
func (pl *Plugin) addLocale(q orm.Query, data map[string]any) (*mks.ActionResult, error) {
	str := func(k string) string { v, _ := data[k].(string); return strings.TrimSpace(v) }
	l := Locale{Code: str("code"), Name: str("name"), Direction: str("direction"), Base: str("base"), Tag: str("tag")}
	errs := mks.ValidationErrors{}
	if !codeRE.MatchString(l.Code) || len(l.Code) > 12 {
		errs["code"] = "A language code is two or three lowercase Latin letters, with an optional part like ar_IQ."
	} else if ok, err := pl.known(q, l.Code); err != nil {
		return nil, err
	} else if ok {
		errs["code"] = "This language already exists."
	}
	if l.Name == "" || utf8.RuneCountInString(l.Name) > 60 {
		errs["name"] = "The language name is required (at most 60 characters)."
	}
	if l.Direction != "rtl" && l.Direction != "ltr" {
		errs["direction"] = "Choose a writing direction."
	}
	if !slices.Contains(Locales, l.Base) {
		errs["base"] = "The base language is Persian or English."
	}
	if l.Tag != "" && !tagRE.MatchString(l.Tag) {
		errs["tag"] = "Invalid language tag (e.g. ckb-IQ)."
	}
	if len(errs) > 0 {
		return nil, errs
	}
	if err := q.Create(&l); err != nil {
		return nil, err
	}
	return mks.Success(fmt.Sprintf("Language \"%s\" added; translate its texts here.", l.Name)), nil
}

// removeLocale deletes an added language and its texts.
func (pl *Plugin) removeLocale(q orm.Query, data map[string]any) (*mks.ActionResult, error) {
	code, _ := data["code"].(string)
	if slices.Contains(Locales, code) {
		return nil, mks.Reject("The panel's built-in languages cannot be deleted.")
	}
	res, err := q.Exec("DELETE FROM "+LocalesTable+" WHERE code = ?", code)
	if err != nil {
		return nil, err
	}
	if res == nil || res.RowsAffected == 0 {
		return nil, mks.Reject("This language was not found.")
	}
	if _, err := q.Exec("DELETE FROM "+Table+" WHERE locale = ?", code); err != nil {
		return nil, err
	}
	return mks.Success("The language and its texts were deleted."), nil
}

// save writes the sent texts of a locale; an empty text goes back to the built-in one.
func (pl *Plugin) save(q orm.Query, locale string, values map[string]any) error {
	if len(values) > 2000 {
		return mks.ValidationErrors{"values": "Too many strings."}
	}
	for key, raw := range values {
		value, ok := raw.(string)
		if !ok || !keyRE.MatchString(key) {
			return mks.ValidationErrors{"values": fmt.Sprintf("Invalid key \"%s\".", key)}
		}
		value = strings.TrimSpace(value)
		if utf8.RuneCountInString(value) > 1000 {
			return mks.ValidationErrors{"values": fmt.Sprintf("The text \"%s\" is longer than 1000 characters.", key)}
		}
		if _, err := q.Exec("DELETE FROM "+Table+" WHERE locale = ? AND name = ?", locale, key); err != nil {
			return err
		}
		if value != "" {
			if err := q.Create(&row{Locale: locale, Name: key, Value: value}); err != nil {
				return err
			}
		}
	}
	return nil
}
