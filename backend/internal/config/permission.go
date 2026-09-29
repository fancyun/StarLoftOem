package config

import (
	"sort"
	"strings"

	"oemrpa/internal/model"
)

// PermissionSpec 后台权限目录项：模块（读权限码）、所属分组、名称，以及是否另有写权限。
// 读/写约定：无后缀码＝可查看该模块；Writable 为 true 时该模块另有 `<code>.write`＝可增删改（写隐含读）。
type PermissionSpec struct {
	Code     string
	Group    string
	Label    string
	Writable bool
}

// 权限分组名（与后台一级分区一致）
const (
	PermissionGroupSys = "平台管理"
	PermissionGroupSMS = "短信服务"
	PermissionGroupFV  = "人脸核验"
)

// PermissionCatalog 全部后台权限模块（勾选项）。模块权限码常量见 model/permission.go；
// Writable 标记该模块是否存在写操作（不存在写操作的不提供 `.write` 权限码）。
func PermissionCatalog() []PermissionSpec {
	return []PermissionSpec{
		{model.PermissionSysDashboard, PermissionGroupSys, "数据统计", false},
		{model.PermissionSysUsers, PermissionGroupSys, "用户管理", true},
		{model.PermissionSysFinance, PermissionGroupSys, "财务管理", true},
		{model.PermissionSysAff, PermissionGroupSys, "推广管理", true},
		{model.PermissionSysAdmins, PermissionGroupSys, "员工管理", true},
		{model.PermissionSysSales, PermissionGroupSys, "销售业绩", false},
		{model.PermissionSysSettings, PermissionGroupSys, "系统设置", true},

		{model.PermissionSmsStats, PermissionGroupSMS, "数据统计", false},
		{model.PermissionSmsSigns, PermissionGroupSMS, "签名审核", true},
		{model.PermissionSmsTemplates, PermissionGroupSMS, "模板审核", true},
		{model.PermissionSmsRecords, PermissionGroupSMS, "发送记录", false},
		{model.PermissionSmsReplies, PermissionGroupSMS, "短信回复", false},
		{model.PermissionSmsPacks, PermissionGroupSMS, "资源包管理", true},
		{model.PermissionSmsProductConfig, PermissionGroupSMS, "产品配置", true},

		{model.PermissionFvStats, PermissionGroupFV, "数据统计", false},
		{model.PermissionFvRecords, PermissionGroupFV, "认证记录", true},
		{model.PermissionFvPacks, PermissionGroupFV, "资源包管理", true},
		{model.PermissionFvProductConfig, PermissionGroupFV, "产品配置", true},
	}
}

// GroupWildcards 分组通配权限码（持有即拥有该分组全部读/写权限），顺序与后台分区一致
func GroupWildcards() []string {
	return []string{model.PermissionSysGroup, model.PermissionSmsGroup, model.PermissionFvGroup}
}

// PermissionCodes 全部合法权限码集合：读权限码 + 可写模块的写权限码 + 分组通配码（用于归一化时过滤未知码）
func PermissionCodes() map[string]bool {
	codes := make(map[string]bool)
	for _, p := range PermissionCatalog() {
		codes[p.Code] = true
		if p.Writable {
			codes[model.WritePermission(p.Code)] = true
		}
	}
	for _, w := range GroupWildcards() {
		codes[w] = true
	}
	return codes
}

// isGroupWildcard 是否为分组通配码（仅一级 token 才算通配，避免读码被当作通配而越权覆盖同模块写码）
func isGroupWildcard(token string) bool {
	for _, w := range GroupWildcards() {
		if token == w {
			return true
		}
	}
	return false
}

// groupOf 取权限码所属分组（即第一个 '.' 前的前缀）；无 '.' 时返回其自身
func groupOf(code string) string {
	if i := strings.Index(code, "."); i >= 0 {
		return code[:i]
	}
	return code
}

// ParsePermissions 拆分权限串为权限码列表（空串返回空列表，all 单独返回一个元素）
func ParsePermissions(perms string) []string {
	out := make([]string, 0)
	for _, v := range strings.Split(perms, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// HasPermission 判断权限串是否包含指定权限码。三类 token 的匹配规则：
//   - `all`：覆盖一切；
//   - 分组通配码（sys / sms / fv）：覆盖同前缀的全部读/写权限码（`sms` 覆盖 `sms.signs` 与 `sms.signs.write`）；
//   - 写权限码 `<模块>.write`：同时满足该模块的读权限码（写隐含读，能改必能看）。
//
// 注意：通配判定只对一级 token 生效，避免读码 `sys.admins` 被当成通配而越权覆盖 `sys.admins.write`。
func HasPermission(perms, code string) bool {
	if code == "" {
		return true
	}
	for _, v := range ParsePermissions(perms) {
		if v == model.PermissionAll || v == code {
			return true
		}
		if isGroupWildcard(v) && strings.HasPrefix(code, v+".") {
			return true
		}
		if strings.HasSuffix(v, model.PermissionWriteSuffix) && strings.TrimSuffix(v, model.PermissionWriteSuffix) == code {
			return true
		}
	}
	return false
}

// NormalizePermissions 归一化权限码列表：过滤未知码、去重、排序，并做两级收敛：
//   - 持有分组通配码时，该分组下的具体权限码不再重复存储（通配已覆盖，含后续新增页面）；
//   - 三个分组通配码齐备等价于 all，直接收敛为 all。
func NormalizePermissions(list []string) string {
	valid := PermissionCodes()
	set := make(map[string]bool)
	hasAll := false
	for _, code := range list {
		code = strings.TrimSpace(code)
		if code == model.PermissionAll {
			hasAll = true
			continue
		}
		if valid[code] {
			set[code] = true
		}
	}
	if hasAll {
		return model.PermissionAll
	}
	wildcards := GroupWildcards()
	for _, w := range wildcards {
		if !set[w] {
			continue
		}
		for code := range set {
			if code == w {
				continue // 通配码自身保留
			}
			if groupOf(code) == w {
				delete(set, code)
			}
		}
	}
	// 三个分组全通配 == 全部权限
	allGroups := true
	for _, w := range wildcards {
		if !set[w] {
			allGroups = false
			break
		}
	}
	if allGroups {
		return model.PermissionAll
	}
	out := make([]string, 0, len(set))
	for code := range set {
		out = append(out, code)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}
