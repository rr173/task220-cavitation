// Package cavitation 事件模块：空化事件的阶段状态机与基于谐波缺口比
// 序列的起始/持续/消退判定。事件生命周期与试验/片段解耦，只消费
// harmonic 包产出的缺口比序列与 threshold 包提供的判定阈值。
package cavitation

import (
	"fmt"

	"task220-cavitation/internal/model"
)

// 阶段流转规则：
//   candidate  -> inception  -> sustained  -> decay
//   candidate  -> rejected（任一非终态阶段均可否决）
//   inception  -> rejected
//   sustained  -> rejected
//   decay      -> rejected
//   inception/sustained/decay 均为单向，不可回退。
var allowedTransitions = map[string]map[string]bool{
	model.EventCandidate: {model.EventInception: true, model.EventRejected: true},
	model.EventInception: {model.EventSustained: true, model.EventRejected: true},
	model.EventSustained: {model.EventDecay: true, model.EventRejected: true},
	model.EventDecay:     {model.EventRejected: true},
	model.EventRejected:  {},
}

// CanTransition 判断阶段 from 是否可流转到 to。
func CanTransition(from, to string) bool {
	if set, ok := allowedTransitions[from]; ok {
		return set[to]
	}
	return false
}

// Transition 执行状态机流转，非法流转返回 ErrInvalidState。
func Transition(e *model.CavitationEvent, to string) error {
	if !CanTransition(e.Stage, to) {
		return fmt.Errorf("%w: %s -> %s", model.ErrInvalidState, e.Stage, to)
	}
	e.Stage = to
	return nil
}

// IsTerminal 判断阶段是否为终态（decay 或 rejected）。
func IsTerminal(stage string) bool {
	return stage == model.EventDecay || stage == model.EventRejected
}
