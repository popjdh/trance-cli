package core

import (
	"errors"
	"strings"
)

// region 枚举定义

type SlotType int

const (
	SlotTypeSingle SlotType = iota
	SlotTypeMulti
)

func toSlotType(str string) (SlotType, error) {
	switch strings.ToLower(str) {
	case "single":
		return SlotTypeSingle, nil
	case "multi":
		return SlotTypeMulti, nil
	default:
		return 0, errors.New("invalid slot type: " + str)
	}
}

type TemplateInstanceStatus int

const (
	TemplateInstanceStatusIncomplete TemplateInstanceStatus = iota
	TemplateInstanceStatusNoInput
	TemplateInstanceStatusComplete
)

// endregion

// region 模板定义

type Template struct {
	Namespace string
	Name      string
	IsRoot    bool
	Literals  []Literal
	Fragments []Fragment
	Slots     []Slot
	Content   string
}

type Literal struct {
	Key     string
	Label   string
	Default *string
	Options []string
}

type Fragment struct {
	Key         string
	When        []string
	Content     string
	LiteralKeys []string
}

type Slot struct {
	Key       string
	Namespace string
	Type      SlotType
	Padding   string
	Gap       string
}

func NewRootSlotInstance() *SlotInstance {
	return &SlotInstance{
		Definition: &Slot{
			Key:       "root",
			Namespace: "root",
			Type:      SlotTypeSingle,
		},
		Instances: []*TemplateInstance{nil},
	}
}

func (tmpl *Template) NewInstance() *TemplateInstance {
	litInsts := make([]LiteralInstance, len(tmpl.Literals))
	for i, lit := range tmpl.Literals {
		litInsts[i] = LiteralInstance{
			Definition: &tmpl.Literals[i],
			Value:      lit.Default,
		}
	}

	slotInsts := make([]SlotInstance, len(tmpl.Slots))
	for i := range tmpl.Slots {
		slotInsts[i] = SlotInstance{
			Definition: &tmpl.Slots[i],
			Instances:  []*TemplateInstance{nil},
		}
	}

	return &TemplateInstance{
		Definition: tmpl,
		Slots:      slotInsts,
		Literals:   litInsts,
	}
}

// endregion

// region 模板实例

type TemplateInstance struct {
	Definition *Template
	Literals   []LiteralInstance
	Slots      []SlotInstance
}

type LiteralInstance struct {
	Definition *Literal
	Value      *string // nil=未填, ptr=有值
}

type SlotInstance struct {
	Definition *Slot
	Instances  []*TemplateInstance
}

func (tmplInst *TemplateInstance) State() TemplateInstanceStatus {
	if len(tmplInst.Literals) == 0 {
		return TemplateInstanceStatusNoInput
	}
	for _, litInst := range tmplInst.Literals {
		if litInst.Value == nil || *litInst.Value == "" {
			return TemplateInstanceStatusIncomplete
		}
	}
	return TemplateInstanceStatusComplete
}

func (slotInst *SlotInstance) Embed(tmplInst *TemplateInstance) {
	if slotInst.Definition.Type == SlotTypeSingle {
		slotInst.Instances = []*TemplateInstance{tmplInst}
	} else {
		slotInst.Instances = append(slotInst.Instances[:len(slotInst.Instances)-1], tmplInst, nil)
	}
}

func (slotInst *SlotInstance) Remove(tmplInst *TemplateInstance) {
	if slotInst.Definition.Type == SlotTypeSingle {
		slotInst.Instances = []*TemplateInstance{nil}
		return
	}

	next := slotInst.Instances[:0]
	for _, inst := range slotInst.Instances {
		if inst != tmplInst {
			next = append(next, inst)
		}
	}

	hasEmpty := false
	for _, inst := range next {
		if inst == nil {
			hasEmpty = true
			break
		}
	}
	if !hasEmpty {
		next = append(next, nil)
	}
	slotInst.Instances = next
}

// endregion
