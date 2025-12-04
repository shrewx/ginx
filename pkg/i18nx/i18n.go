package i18nx

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
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
			bundle.RegisterUnmarshalFunc("yml", yaml.Unmarshal)
		default:
			panic(fmt.Errorf("unmarshal type %s is not support", c.UnmarshalType))
		}
	}

	// 加载多个路径的文件
	if len(c.Paths) > 0 {
		for _, path := range c.Paths {
			if path == "" {
				continue
			}

			info, err := os.Stat(path)
			if err != nil {
				continue
			}

			if info.IsDir() {
				// 如果是目录，遍历加载目录下的所有文件
				err := loadPathFiles(path, c.UnmarshalType)
				if err != nil {
					panic(fmt.Errorf("load i18n files from path %s fail, err: %s", path, err.Error()))
				}
			} else if info.Mode().IsRegular() {
				if err := loadMessageFile(path, c.UnmarshalType); err != nil {
					panic(fmt.Errorf("load i18n file from path %s fail, err: %s", path, err.Error()))
				}
			}
		}
	}

	localize.localizers = make(map[string]*i18n.Localizer)
	for _, lang := range langs {
		localize.localizers[lang] = i18n.NewLocalizer(bundle, lang)
	}
}

func loadI18nFromYAML(bundle *i18n.Bundle, path string) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lang, err := parseLangFromContent(buf)
	if err != nil {
		return err
	}

	_, err = bundle.ParseMessageFileBytes(buf, fmt.Sprintf("%s.yaml", lang))
	return err
}

func parseLangFromContent(buf []byte) (string, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal(buf, &data); err != nil {
		return "", err
	}
	for lang := range data {
		return lang, nil
	}
	return "", errors.New("no language key found")
}

// loadPathFiles 加载指定路径下的所有消息文件
func loadPathFiles(path string, unmarshalType string) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filePath == path {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if err := loadMessageFile(filePath, unmarshalType); err != nil {
			panic(err)
		}

		return nil
	})
}

func loadMessageFile(filePath, unmarshalType string) error {
	if _, err := bundle.LoadMessageFile(filePath); err != nil {
		if strings.ToUpper(unmarshalType) == "YAML" {
			return loadI18nFromYAML(bundle, filePath)
		}
		return err
	}
	return nil
}

// AddPath 运行时添加单个路径
func AddPath(path string, unmarshalType string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path not exist: %s", path)
	}

	if info.IsDir() {
		// 如果是目录，遍历加载目录下的所有文件
		return loadPathFiles(path, unmarshalType)
	} else if info.Mode().IsRegular() {
		// 如果是文件，直接加载
		return loadMessageFile(path, unmarshalType)
	}

	return fmt.Errorf("path is not a file or directory: %s", path)
}

// AddPaths 运行时批量添加路径
func AddPaths(paths []string, unmarshalType string) error {
	for _, path := range paths {
		if err := AddPath(path, unmarshalType); err != nil {
			return err
		}
	}
	return nil
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
		logx.Errorf("localize error message fail, err:%s", err.Error())
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
