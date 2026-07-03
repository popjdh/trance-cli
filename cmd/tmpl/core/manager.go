package core

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type TemplateManager struct {
	templates []Template
}

func NewTemplateManager(dir string) (*TemplateManager, error) {
	templates, err := collectTemplates(dir)
	if err != nil {
		return nil, err
	}
	return &TemplateManager{templates: templates}, nil
}

func (m *TemplateManager) List(namespace string) []*Template {
	var result []*Template
	for i, template := range m.templates {
		if template.IsRoot && namespace == "root" {
			result = append(result, &m.templates[i])
		} else if !template.IsRoot && template.Namespace == namespace {
			result = append(result, &m.templates[i])
		}
	}
	return result
}

func (m *TemplateManager) NewInstance(templateDef *Template) *TemplateInstance {
	literals := make([]LiteralInstance, len(templateDef.Literals))
	for i, l := range templateDef.Literals {
		literals[i] = LiteralInstance{
			Definition: &templateDef.Literals[i],
			Value:      l.Default,
		}
	}

	slots := make([]SlotInstance, len(templateDef.Slots))
	for i := range templateDef.Slots {
		slots[i] = SlotInstance{
			Definition: &templateDef.Slots[i],
			Instances:  []*TemplateInstance{nil},
		}
	}

	return &TemplateInstance{
		Definition: templateDef,
		Literals:   literals,
		Slots:      slots,
	}
}

func (m *TemplateManager) Render(template *TemplateInstance, withMarker bool) string {
	if template == nil {
		return ""
	}
	result := m.render(template, 1, withMarker)
	if withMarker {
		// 包裹顶层标记, 保留 ⟧ ⟫ 去除尾部空白
		wrapped := fmt.Sprintf("⟦1|%s⟧", result)
		re := regexp.MustCompile(`([\s⟧⟫]*)$`)
		cleaned := re.ReplaceAllStringFunc(wrapped, func(match string) string {
			var b strings.Builder
			for _, r := range match {
				if !unicode.IsSpace(r) {
					b.WriteRune(r)
				}
			}
			return b.String()
		})
		return cleaned
	}
	return strings.TrimRightFunc(result, unicode.IsSpace)
}

func (m *TemplateManager) render(template *TemplateInstance, level int, withMarker bool) string {
	if template == nil || template.Definition == nil {
		return ""
	}
	result := template.Definition.Content
	literalKeyMap := map[string]LiteralInstance{}
	for _, literal := range template.Literals {
		literalKeyMap[literal.Definition.Key] = literal
	}

	// 替换 literal
	for _, literal := range template.Literals {
		replacement := renderLiteralValue(literal, withMarker)
		result = strings.ReplaceAll(result, "{{"+literal.Definition.Key+"}}", replacement)
	}

	// 替换 fragment
	for _, fragment := range template.Definition.Fragments {
		placeholder := "{{fragment:" + fragment.Key + "}}"
		replacement, omitted := m.renderFragmentValue(fragment, literalKeyMap, withMarker)
		if omitted {
			result = removePlaceholder(result, placeholder)
			continue
		}
		result = strings.ReplaceAll(result, placeholder, replacement)
	}

	// 替换 slot
	for _, slot := range template.Slots {
		slotKey := slot.Definition.Key
		slotType := slot.Definition.Type
		slotPadding := slot.Definition.Padding
		slotGap := slot.Definition.Gap

		// 空 slot → 移除行 (检查是否有非 nil 实例)
		hasInstance := false
		for _, inst := range slot.Instances {
			if inst != nil {
				hasInstance = true
				break
			}
		}
		if !hasInstance {
			var placeholder string
			if slotType == SlotTypeMulti {
				placeholder = "{{slots:" + slotKey + "}}"
			} else {
				placeholder = "{{slot:" + slotKey + "}}"
			}
			result = removePlaceholder(result, placeholder)
			continue
		}

		// 递归渲染每个子节点 (跳过 nil 空位)
		var rendered []string
		for _, child := range slot.Instances {
			if child == nil {
				continue
			}
			rendered = append(rendered, m.render(child, level+1, withMarker))
		}

		// 加 padding
		if slotPadding != "" {
			for i, r := range rendered {
				lines := strings.Split(r, "\n")
				for j, line := range lines {
					if j == 0 {
						lines[j] = slotPadding + line
					} else if line != "" {
						lines[j] = slotPadding + line
					}
				}
				rendered[i] = strings.Join(lines, "\n")
			}
		}

		// 加标记
		if withMarker {
			for i, r := range rendered {
				rendered[i] = fmt.Sprintf("⟦%d|%s⟧", level+1, r)
			}
		}

		// 拼接
		joined := strings.Join(rendered, slotGap)

		// 替换占位符
		var placeholder string
		if slotType == SlotTypeMulti {
			placeholder = "{{slots:" + slotKey + "}}"
		} else {
			placeholder = "{{slot:" + slotKey + "}}"
		}
		result = strings.ReplaceAll(result, placeholder, joined)
	}

	return result
}

func renderLiteralValue(literal LiteralInstance, withMarker bool) string {
	if literal.Value == nil {
		if withMarker {
			return "⟪x|null⟫"
		}
		return "null"
	}
	if withMarker {
		return fmt.Sprintf("⟪=|%s⟫", *literal.Value)
	}
	return *literal.Value
}

func (m *TemplateManager) renderFragmentValue(fragment Fragment, literalKeyMap map[string]LiteralInstance, withMarker bool) (string, bool) {
	for _, key := range fragment.When {
		literal, ok := literalKeyMap[key]
		if !ok || literal.Value == nil {
			return "", true
		}
	}

	result := fragment.Content
	for _, key := range fragment.LiteralKeys {
		literal, ok := literalKeyMap[key]
		if !ok {
			continue
		}
		result = strings.ReplaceAll(result, "{{"+key+"}}", renderLiteralValue(literal, withMarker))
	}

	if withMarker {
		result = fmt.Sprintf("⟦f|%s⟧", result)
	}
	return result, false
}

func removePlaceholder(content string, placeholder string) string {
	safePlaceholder := regexp.QuoteMeta(placeholder)
	re0 := regexp.MustCompile(`^[\p{Zs}\t]*` + safePlaceholder + `[\p{Zs}\t]*\n`)
	content = re0.ReplaceAllString(content, "")
	re1 := regexp.MustCompile(`\n[\p{Zs}\t]*` + safePlaceholder + `[\p{Zs}\t]*\n`)
	content = re1.ReplaceAllString(content, "\n")
	re2 := regexp.MustCompile(`\n[\p{Zs}\t]*` + safePlaceholder + `[\p{Zs}\t]*$`)
	content = re2.ReplaceAllString(content, "")
	re3 := regexp.MustCompile(`^[\p{Zs}\t]*` + safePlaceholder + `[\p{Zs}\t]*$`)
	content = re3.ReplaceAllString(content, "")
	return strings.ReplaceAll(content, placeholder, "")
}
