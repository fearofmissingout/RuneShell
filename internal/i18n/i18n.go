package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

const (
	LangZhCN        = "zh-CN"
	LangEnUS        = "en-US"
	DefaultLanguage = LangZhCN
)

type Args map[string]any

//go:embed locales/*.json
var localeFS embed.FS

var (
	loadOnce sync.Once
	catalogs map[string]map[string]string
	loadErr  error
)

func SupportedLanguages() []string {
	return []string{LangZhCN, LangEnUS}
}

func NormalizeLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "", "zh", "zh-cn", "zh_hans", "zh-hans":
		return LangZhCN
	case "en", "en-us", "en_us":
		return LangEnUS
	default:
		return DefaultLanguage
	}
}

func Text(lang string, key string, args Args) string {
	loadCatalogs()
	normalized := NormalizeLanguage(lang)
	value := lookup(normalized, key)
	if value == "" {
		value = lookup(DefaultLanguage, key)
	}
	if value == "" {
		return key
	}
	return interpolate(value, args)
}

func loadCatalogs() {
	loadOnce.Do(func() {
		catalogs = map[string]map[string]string{}
		for _, lang := range SupportedLanguages() {
			filename := path.Join("locales", lang+".json")
			data, err := localeFS.ReadFile(filename)
			if err != nil {
				loadErr = err
				return
			}
			var raw map[string]string
			if err := json.Unmarshal(data, &raw); err != nil {
				loadErr = err
				return
			}
			catalogs[lang] = raw
		}
	})
}

func lookup(lang string, key string) string {
	if loadErr != nil {
		return ""
	}
	if catalog, ok := catalogs[lang]; ok {
		return catalog[key]
	}
	return ""
}

func interpolate(value string, args Args) string {
	if len(args) == 0 {
		return value
	}
	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	replacements := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		replacements = append(replacements, "{"+key+"}", fmt.Sprint(args[key]))
	}
	return strings.NewReplacer(replacements...).Replace(value)
}
