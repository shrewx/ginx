package i18nx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/logx"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

type I18nMessage interface {
	Localize(manager *Localize, lang string) I18nMessage
	Key() string
	Value() string
	Prefix() string
}

func RegisterHooks(f func()) {
	registerHooks = append(registerHooks, f)
}

func AddMessages(lang string, messages []*i18n.Message) {
	t, err := language.Parse(lang)
	if err != nil {
		panic(fmt.Errorf("lang is not support, err: %s", err.Error()))
	}
	err = addMessages(t, messages...)
	if err != nil {
		panic(fmt.Errorf("add message failed, err: %s", err.Error()))
	}
}

func addMessages(tag language.Tag, messages ...*i18n.Message) error {
	return bundle.AddMessages(tag, messages...)
}

var (
	defaultLang   = language.Chinese
	bundle        = i18n.NewBundle(defaultLang)
	registerHooks = make([]func(), 0)
	localize      = &Localize{}
)

func Instance() *Localize {
	return localize
}

func Load(c *conf.I18N) {
	var langs []string
	if len(c.Langs) == 0 {
		c.Langs = []string{"zh", "en"}
	}

	for _, lang := range c.Langs {
		t, err := language.Parse(lang)
		if err != nil {
			panic(fmt.Errorf("lang is not support, err: %s", err.Error()))
		}
		langs = append(langs, t.String())
	}

	for _, f := range registerHooks {
		f()
	}

	if c.UnmarshalType != "" {
		switch strings.ToUpper(c.UnmarshalType) {
		case "TOML":
			bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
		case "JSON":
			bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
		case "YAML":
			bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
		default:
			panic(fmt.Errorf("unmarshal type %s is not support", c.UnmarshalType))
		}
	}

	if c.Path != "" && pathExist(c.Path) {
		err := filepath.Walk(c.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path == c.Path {
				return nil
			}
			bundle.MustLoadMessageFile(path)
			return nil
		})
		if err != nil {
			panic(fmt.Errorf("load i18n files fail, err: %s", err.Error()))
		}
	}
	localize.localizers = make(map[string]*i18n.Localizer)
	for _, lang := range langs {
		localize.localizers[lang] = i18n.NewLocalizer(bundle, lang)
	}
}

type Localize struct {
	localizers map[string]*i18n.Localizer
}

// getLocalizer ensures a non-nil localizer.
// If not initialized via Load, it builds a temporary one
// using the requested lang with defaultLang as fallback.
func (m *Localize) getLocalizer(lang string) *i18n.Localizer {
	// Always respect the requested lang to avoid mismatches when MessageID embeds lang
	if m != nil && m.localizers != nil && m.localizers[lang] != nil {
		return m.localizers[lang]
	}
	return i18n.NewLocalizer(bundle, lang)
}

func (m *Localize) LocalizeData(lang, key string, data map[string]interface{}) (string, error) {
	return m.getLocalizer(lang).Localize(&i18n.LocalizeConfig{
		MessageID:    fmt.Sprintf("%s.%s", lang, key),
		TemplateData: data,
	})
}

func (m *Localize) Localize(lang, key string) (string, error) {
	return m.getLocalizer(lang).Localize(&i18n.LocalizeConfig{
		MessageID: fmt.Sprintf("%s.%s", lang, key),
	})
}

func pathExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		// 路径存在
		return true
	}
	return false
}

type Message struct {
	// 类型
	T string `json:"type"`
	// 名称
	K string `json:"key"`
	// 前缀
	P string `json:"prefix"`
	// 消息
	Message string `json:"message"`
	// 消息
	Langs map[string]string `json:"-"`
}

func NewMessage(key, prefix string) *Message {
	return &Message{
		K: key,
		P: prefix,
	}
}

func (m *Message) Localize(manager *Localize, lang string) I18nMessage {
	key := m.K
	if m.P != "" {
		key = m.P + "." + m.K
	}
	message, err := manager.LocalizeData(lang, key, map[string]interface{}{})
	if err != nil {
		logx.Error("localize error message fail, err:%s", err.Error())
		return m
	}
	m.Message = message
	return m
}

func (m *Message) Key() string {
	return m.K
}

func (m *Message) Value() string {
	return m.Message
}

func (m *Message) Prefix() string {
	return m.P
}
