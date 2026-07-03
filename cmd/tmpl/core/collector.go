package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml/v2"
)

// region TOML 结构体

type tomlDocument struct {
	Template []tomlTemplate `toml:"template"`
}

type tomlTemplate struct {
	Namespace string         `toml:"namespace"`
	Name      string         `toml:"name"`
	IsRoot    bool           `toml:"is_root"`
	Literals  []tomlLiteral  `toml:"literals"`
	Fragments []tomlFragment `toml:"fragments"`
	Slots     []tomlSlot     `toml:"slots"`
	Content   string         `toml:"content"`
}

type tomlLiteral struct {
	Key     string   `toml:"key"`
	Label   string   `toml:"label"`
	Default *string  `toml:"default"`
	Options []string `toml:"options"`
}

type tomlFragment struct {
	Key     string `toml:"key"`
	When    any    `toml:"when"`
	Content string `toml:"content"`
}

type tomlSlot struct {
	Key       string `toml:"key"`
	Namespace string `toml:"namespace"`
	Type      string `toml:"type"`
	Padding   any    `toml:"padding"` // int 对应空格个数, string 对应自由设置
	Gap       any    `toml:"gap"`     // int 对应换行格式, string 对应自由设置
}

// endregion

func collectTemplates(dir string) ([]Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("模板目录读取失败: %s\n%w", dir, err)
	}

	var result []Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		subResult, err := parseFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("模板文件解析错误: %s\n%w", entry.Name(), err)
		}
		result = append(result, subResult...)
	}
	return result, nil
}

func parseFile(path string) ([]Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc tomlDocument
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	tmpls := make([]Template, 0, len(doc.Template))
	for _, rawTmpl := range doc.Template {
		tmpl, err := parseTomlTemplate(rawTmpl)
		if err != nil {
			return nil, fmt.Errorf("模板解析失败: %s\n%w", path, err)
		}
		tmpls = append(tmpls, tmpl)
	}
	return tmpls, nil
}

func parseTomlTemplate(rawTmpl tomlTemplate) (Template, error) {
	name := rawTmpl.Name
	if name == "" {
		if rawTmpl.IsRoot {
			name = rawTmpl.Namespace
		} else {
			name = "default"
		}
	}

	lits := make([]Literal, 0, len(rawTmpl.Literals))
	litKeySet := map[string]struct{}{}
	for _, rawLit := range rawTmpl.Literals {
		lit, err := parseTomlLiteral(rawLit)
		if err != nil {
			return Template{}, err
		}
		if lit.Key == "" {
			return Template{}, fmt.Errorf("Literal Key 不能为空")
		}
		if _, ok := litKeySet[lit.Key]; ok {
			return Template{}, fmt.Errorf("Literal Key 重复: %s", lit.Key)
		}
		litKeySet[lit.Key] = struct{}{}
		lits = append(lits, lit)
	}

	frags := make([]Fragment, 0, len(rawTmpl.Fragments))
	fragKeySet := map[string]struct{}{}
	for _, rawFrag := range rawTmpl.Fragments {
		frag, err := parseTomlFragment(rawFrag, litKeySet)
		if err != nil {
			return Template{}, err
		}
		if frag.Key == "" {
			return Template{}, fmt.Errorf("Fragment Key 不能为空")
		}
		if _, ok := fragKeySet[frag.Key]; ok {
			return Template{}, fmt.Errorf("Fragment Key 重复: %s", frag.Key)
		}
		fragKeySet[frag.Key] = struct{}{}
		frags = append(frags, frag)
	}

	slots := make([]Slot, 0, len(rawTmpl.Slots))
	slotKeySet := map[string]struct{}{}
	for _, rawSlot := range rawTmpl.Slots {
		slot, err := parseTomlSlot(rawSlot, rawTmpl)
		if err != nil {
			return Template{}, err
		}
		if slot.Namespace == "" {
			return Template{}, fmt.Errorf("Slot Namespace 不能为空")
		}
		if slot.Key == "" {
			return Template{}, fmt.Errorf("Slot Key 不能为空")
		}
		if _, ok := slotKeySet[slot.Key]; ok {
			return Template{}, fmt.Errorf("Slot Key 重复: %s", slot.Key)
		}
		slotKeySet[slot.Key] = struct{}{}
		slots = append(slots, slot)
	}

	var (
		slotRe     = regexp.MustCompile(`{{[\p{Zs}\t]*(slots?)[\p{Zs}\t]*:[\p{Zs}\t]*(>?[\pL\pN_.-]+)[\p{Zs}\t]*}}`)
		fragmentRe = regexp.MustCompile(`{{[\p{Zs}\t]*fragment[\p{Zs}\t]*:[\p{Zs}\t]*([\pL\pN_]+)[\p{Zs}\t]*}}`)
		litRe      = regexp.MustCompile(`{{[\p{Zs}\t]*([\pL\pN_]+)[\p{Zs}\t]*}}`)
		abbrSlotRe = regexp.MustCompile(`{{(slots?):>([\pL\pN_.-]+)}}`)
	)
	content := rawTmpl.Content
	content = slotRe.ReplaceAllString(content, "{{$1:$2}}")
	content = fragmentRe.ReplaceAllString(content, "{{fragment:$1}}")
	content = litRe.ReplaceAllString(content, "{{$1}}")
	content = abbrSlotRe.ReplaceAllString(content, fmt.Sprintf("{{$1:%s.$2}}", rawTmpl.Namespace))
	content = strings.TrimRightFunc(content, unicode.IsSpace)

	return Template{
		Namespace: rawTmpl.Namespace,
		Name:      name,
		IsRoot:    rawTmpl.IsRoot,
		Literals:  lits,
		Fragments: frags,
		Slots:     slots,
		Content:   content,
	}, nil
}

func parseTomlLiteral(rawLit tomlLiteral) (Literal, error) {
	label := rawLit.Label
	if label == "" {
		label = rawLit.Key
	}

	options := rawLit.Options
	if options == nil {
		options = []string{}
	}

	return Literal{
		Key:     rawLit.Key,
		Label:   label,
		Default: rawLit.Default,
		Options: options,
	}, nil
}

func parseTomlFragment(rawFrag tomlFragment, globalLiteralKeySet map[string]struct{}) (Fragment, error) {
	var when []string
	if rawFrag.When != nil {
		switch v := rawFrag.When.(type) {
		case string:
			when = []string{v}
		case []string:
			when = v
		case []any:
			for _, item := range v {
				if str, ok := item.(string); ok {
					when = append(when, str)
				} else {
					return Fragment{}, fmt.Errorf("Fragment When 类型错误: %T", item)
				}
			}
		default:
			return Fragment{}, fmt.Errorf("Fragment When 类型错误: %T", rawFrag.When)
		}
	}
	for _, literalKey := range when {
		if _, ok := globalLiteralKeySet[literalKey]; !ok {
			return Fragment{}, fmt.Errorf("Fragment When 引用未知 Literal: %s", literalKey)
		}
	}

	content := rawFrag.Content
	litRe := regexp.MustCompile(`{{[\p{Zs}\t]*([\pL\pN_]+)[\p{Zs}\t]*}}`)
	content = litRe.ReplaceAllString(content, "{{$1}}")
	content = strings.TrimRightFunc(content, unicode.IsSpace)

	var literalKeys []string
	literalKeySet := map[string]struct{}{}
	compactLitRe := regexp.MustCompile(`{{([\pL\pN_]+)}}`)
	for _, match := range compactLitRe.FindAllStringSubmatch(content, -1) {
		literalKey := match[1]
		_, ok := globalLiteralKeySet[literalKey]
		_, seen := literalKeySet[literalKey]
		// 未知 LiteralKey 引用视为合法, 看作一般文本
		if ok && !seen {
			literalKeys = append(literalKeys, match[1])
			literalKeySet[literalKey] = struct{}{}
		}
	}
	for _, literalKey := range when {
		if _, seen := literalKeySet[literalKey]; !seen {
			literalKeys = append(literalKeys, literalKey)
			literalKeySet[literalKey] = struct{}{}
		}
	}

	return Fragment{
		Key:         rawFrag.Key,
		When:        when,
		Content:     content,
		LiteralKeys: literalKeys,
	}, nil
}

func parseTomlSlot(rawSlot tomlSlot, rawTmpl tomlTemplate) (Slot, error) {
	namespace := rawSlot.Namespace
	if strings.HasPrefix(namespace, ">") {
		namespace = rawTmpl.Namespace + "." + namespace[1:]
	}
	key := rawSlot.Key
	if key == "" {
		key = namespace
	} else if strings.HasPrefix(key, ">") {
		key = rawTmpl.Namespace + "." + key[1:]
	}

	slotType, err := toSlotType(rawSlot.Type)
	if err != nil {
		return Slot{}, err
	}

	var padding string
	if rawSlot.Padding != nil {
		switch v := rawSlot.Padding.(type) {
		case int64:
			padding = strings.Repeat(" ", int(v))
		case string:
			padding = v
		default:
			return Slot{}, fmt.Errorf("Slot Padding 类型错误: %T", rawSlot.Padding)
		}
	}

	var gap string
	if rawSlot.Gap != nil {
		switch v := rawSlot.Gap.(type) {
		case int64:
			gap = strings.Repeat("\n", int(v))
		case string:
			gap = v
		default:
			return Slot{}, fmt.Errorf("Slot Gap 类型错误: %T", rawSlot.Gap)
		}
	}

	return Slot{
		Key:       key,
		Namespace: namespace,
		Type:      slotType,
		Padding:   padding,
		Gap:       gap,
	}, nil
}
