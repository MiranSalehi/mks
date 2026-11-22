package mks

import (
	"strings"

	"github.com/goravel/framework/contracts/http"
)

// texts are the user-facing messages of one locale. Messages meant for developers (for
// example "sorting by x is not allowed") stay in English.
type texts struct {
	invalidData        string
	notFillable        string
	invalidType        string
	invalidDate        string
	required           string
	invalidCredentials string
	tooManyLogins      string
	unauthenticated    string
	csrfMismatch       string
	resourceNotFound   string
	recordNotFound     string
	duplicate          string
	serverError        string
	inUse              string
	forbidden          string
	notInTenant        string
	deleted            string // %s is the formatted count
	actionDone         string
	uploadTooLarge     string // %s is the size limit
	uploadType         string
	uploadInvalid      string
	tooManyFiles       string
	megabytes          string
	kilobytes          string
	// rules are validation messages by rule name. They do not name the field: the UI shows
	// each error under its own field, and field labels live in the frontend definitions.
	rules map[string]string
	// digits, when set, replaces 0-9 in messages (Persian digits for fa).
	digits *strings.Replacer
}

var english = &texts{
	invalidData:        "The given data was invalid.",
	notFillable:        "This field cannot be set.",
	invalidType:        "Invalid value type.",
	invalidDate:        "Must be an ISO 8601 date.",
	required:           "This field is required.",
	invalidCredentials: "These credentials do not match our records.",
	tooManyLogins:      "Too many login attempts. Please try again in a minute.",
	unauthenticated:    "Unauthenticated.",
	csrfMismatch:       "CSRF token mismatch.",
	resourceNotFound:   "Resource not found.",
	recordNotFound:     "Record not found.",
	duplicate:          "A record with the same unique value already exists.",
	serverError:        "Server error.",
	inUse:              "This record is used elsewhere and cannot be deleted.",
	forbidden:          "You are not allowed to do this.",
	notInTenant:        "This item is not in the current workspace.",
	deleted:            "%s record(s) deleted.",
	actionDone:         "Done.",
	uploadTooLarge:     "The file may not be larger than %s.",
	uploadType:         "This file type is not allowed.",
	uploadInvalid:      "The upload expired or is invalid; please upload the file again.",
	tooManyFiles:       "Too many files.",
	megabytes:          "MB",
	kilobytes:          "KB",
	// nil: Goravel's own English messages are used.
}

var persian = &texts{
	invalidData:        "اطلاعات واردشده معتبر نیست.",
	notFillable:        "این فیلد قابل تغییر نیست.",
	invalidType:        "نوع مقدار معتبر نیست.",
	invalidDate:        "تاریخ معتبر نیست.",
	required:           "این فیلد الزامی است.",
	invalidCredentials: "ایمیل یا رمز عبور اشتباه است.",
	tooManyLogins:      "تعداد تلاش‌های ورود زیاد است. یک دقیقه دیگر دوباره امتحان کنید.",
	unauthenticated:    "ابتدا وارد شوید.",
	csrfMismatch:       "نشست شما منقضی شده است. صفحه را دوباره بارگذاری کنید.",
	resourceNotFound:   "این بخش پیدا نشد.",
	recordNotFound:     "رکورد پیدا نشد.",
	duplicate:          "رکوردی با همین مقدار یکتا وجود دارد.",
	serverError:        "خطای سرور.",
	inUse:              "این رکورد در جای دیگری استفاده شده و قابل حذف نیست.",
	forbidden:          "شما اجازه‌ی این کار را ندارید.",
	notInTenant:        "این مورد در فضای کاری فعلی نیست.",
	deleted:            "%s رکورد حذف شد.",
	actionDone:         "انجام شد.",
	uploadTooLarge:     "حجم فایل نباید بیشتر از %s باشد.",
	uploadType:         "این نوع فایل مجاز نیست.",
	uploadInvalid:      "فایل منقضی شده یا معتبر نیست؛ دوباره بارگذاری کنید.",
	tooManyFiles:       "تعداد فایل‌ها بیشتر از حد مجاز است.",
	megabytes:          "مگابایت",
	kilobytes:          "کیلوبایت",
	digits:             strings.NewReplacer("0", "۰", "1", "۱", "2", "۲", "3", "۳", "4", "۴", "5", "۵", "6", "۶", "7", "۷", "8", "۸", "9", "۹"),
	rules: map[string]string{
		"required":             "این فیلد الزامی است.",
		"required_if":          "این فیلد الزامی است.",
		"required_unless":      "این فیلد الزامی است.",
		"required_with":        "این فیلد الزامی است.",
		"required_with_all":    "این فیلد الزامی است.",
		"required_without":     "این فیلد الزامی است.",
		"required_without_all": "این فیلد الزامی است.",
		"filled":               "این فیلد نباید خالی باشد.",
		"accepted":             "این گزینه باید پذیرفته شود.",
		"max_len":              "حداکثر :max کاراکتر مجاز است.",
		"min_len":              "حداقل :min کاراکتر لازم است.",
		"len":                  "باید دقیقاً :size کاراکتر باشد.",
		"max":                  "نباید بیشتر از :max باشد.",
		"min":                  "نباید کمتر از :min باشد.",
		"between":              "باید بین :min و :max باشد.",
		"size":                 "اندازه باید :size باشد.",
		"gt":                   "باید بیشتر از :value باشد.",
		"gte":                  "باید بیشتر یا مساوی :value باشد.",
		"lt":                   "باید کمتر از :value باشد.",
		"lte":                  "باید کمتر یا مساوی :value باشد.",
		"digits":               "باید :digits رقم باشد.",
		"unique":               "این مقدار قبلاً ثبت شده است.",
		"exists":               "مقدار انتخاب‌شده وجود ندارد.",
		"in":                   "مقدار انتخاب‌شده معتبر نیست.",
		"not_in":               "مقدار انتخاب‌شده معتبر نیست.",
		"regex":                "قالب این فیلد معتبر نیست.",
		"not_regex":            "قالب این فیلد معتبر نیست.",
		"email":                "یک ایمیل معتبر وارد کنید.",
		"url":                  "یک آدرس اینترنتی معتبر وارد کنید.",
		"full_url":             "یک آدرس اینترنتی کامل وارد کنید.",
		"numeric":              "باید عدد باشد.",
		"number":               "باید عدد باشد.",
		"integer":              "باید عدد صحیح باشد.",
		"int":                  "باید عدد صحیح باشد.",
		"uint":                 "باید عدد صحیح مثبت باشد.",
		"float":                "باید عدد باشد.",
		"boolean":              "باید بله یا خیر باشد.",
		"bool":                 "باید بله یا خیر باشد.",
		"string":               "باید متن باشد.",
		"array":                "باید فهرست باشد.",
		"slice":                "باید فهرست باشد.",
		"date":                 "تاریخ معتبر نیست.",
		"date_format":          "تاریخ باید با قالب :format باشد.",
		"confirmed":            "تکرار این فیلد مطابقت ندارد.",
		"same":                 "باید با فیلد دیگر یکسان باشد.",
		"different":            "باید با فیلد دیگر متفاوت باشد.",
		"alpha":                "فقط حروف مجاز است.",
		"alpha_num":            "فقط حروف و اعداد مجاز است.",
		"alpha_dash":           "فقط حروف، اعداد، خط تیره و زیرخط مجاز است.",
		"lowercase":            "باید با حروف کوچک باشد.",
		"uppercase":            "باید با حروف بزرگ باشد.",
		"starts_with":          "شروع این مقدار معتبر نیست.",
		"ends_with":            "پایان این مقدار معتبر نیست.",
		"uuid":                 "شناسه‌ی UUID معتبر نیست.",
		"ip":                   "آدرس IP معتبر نیست.",
		"json":                 "JSON معتبر نیست.",
		"hex_color":            "کد رنگ معتبر نیست.",
	},
}

func textsFor(locale string) *texts {
	if strings.HasPrefix(locale, "fa") {
		return persian
	}
	return english
}

func requestTexts(locale string, fallback *texts) *texts {
	if locale == "" {
		return fallback
	}
	return textsFor(locale)
}

// localize converts digits for locales that need it.
func (t *texts) localize(s string) string {
	if t.digits == nil {
		return s
	}
	return t.digits.Replace(s)
}

// LocaleHeader carries the UI's language, so messages come back in it.
const LocaleHeader = "X-Panel-Locale"

// tr is the texts for the request's language (LocaleHeader), or the panel's default.
func (p *Panel) tr(ctx http.Context) *texts {
	if ctx == nil {
		return p.t
	}
	return requestTexts(ctx.Request().Header(LocaleHeader, ""), p.t)
}
