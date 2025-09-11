package i18nx

import (
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/logx"
	"golang.org/x/text/language"
	"os"
	"path/filepath"
	"strings"
)

type I18nMessage interface {
	Localize(manager *Localize, lang string) I18nMessage
	Value() string
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
	localizes := make(map[string]*i18n.Localizer)
	for _, lang := range langs {
		localizes[lang] = i18n.NewLocalizer(bundle, lang)
	}

	localize.localizers = localizes
}

type Localize struct {
	localizers map[string]*i18n.Localizer
}

func (m *Localize) LocalizeData(lang, key string, data map[string]interface{}) (string, error) {
	return m.localizers[lang].Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: data,
	})
}

func (m *Localize) Localize(lang, key string) (string, error) {
	return m.localizers[lang].Localize(&i18n.LocalizeConfig{
		MessageID: key,
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
	// 名称
	Key string `json:"key"`
	// 码
	Code int64 `json:"code"`
	// 消息
	Message string `json:"messages"`
	// 消息
	Langs map[string]string `json:"-"`
}

func NewMessage(key string, code int64) *Message {
	return &Message{
		Key:  key,
		Code: code,
	}
}

func (m *Message) Localize(manager *Localize, lang string) I18nMessage {
	message, err := manager.LocalizeData(lang, m.Key, map[string]interface{}{})
	if err != nil {
		logx.Error("localize error message fail, err:%s", err.Error())
		return m
	}
	m.Message = message
	return m
}

func (m *Message) Value() string {
	return m.Message
}
