package upstream

import "errors"

// ErrFinAuthDataDestroyed 上游核验结果已不可取回（核验数据已销毁或有效期已用尽）。
// 只代表结果无法取回，不代表核验失败：业务层据此把订单终结为「超时结束（未完成核身，不计费已退款）」。
var ErrFinAuthDataDestroyed = errors.New("finauth data destroyed")