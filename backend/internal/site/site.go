// Package site 统一维护平台各站点的域名拼装规则。
// 主域由后台配置（BRAND_ROOT_DOMAIN）在启动时注入，所有站点域名均为「子域前缀.主域」形式。
package site

import "strings"

// RootDomain 平台主域（默认值；启动时经 SetRootDomain 由后台配置覆盖）
var RootDomain = "oem.example.com"

// SetRootDomain 设置平台主域：去协议/端口/路径并转小写，空值忽略
func SetRootDomain(root string) {
	if h := normalizeHost(root); h != "" {
		RootDomain = h
	}
}

// 站点子域前缀
const (
	KindPortal  = "www"     // 门户
	KindConsole = "console" // 控制台
	KindAPI     = "api"     // 下游 API
	KindImg     = "img"     // 对外图片
	KindService = "service" // 服务网页承接
)

// knownPrefixes 已知站点子域前缀（解析访问域名归属时需剥离）
var knownPrefixes = []string{KindPortal, KindConsole, KindAPI, KindImg, KindService}

// sitePrefixes 站点子域前缀（不含门户 www）：登记 推广 域名时需剥离，
// 这些站点域名由平台按主域固定拼出，不应写入登记值
var sitePrefixes = []string{KindConsole, KindAPI, KindImg, KindService}

// IsSitePrefix 判断标签是否为站点子域前缀（console/api/img/service，不含 www）
func IsSitePrefix(label string) bool {
	for _, p := range sitePrefixes {
		if label == p {
			return true
		}
	}
	return false
}

// Hosts 一组站点域名（平台默认站点或某品牌下的同名站点）
type Hosts struct {
	Portal  string // 门户域名
	Console string // 控制台域名
	API     string // 下游 API 域名
	Img     string // 对外图片域名
	Service string // 服务网页承接域名
}

// Platform 返回平台默认站点域名
func Platform() Hosts {
	return ForRoot(RootDomain)
}

// ForRoot 按主域拼出各站点域名：主域为平台主域时返回平台默认站点，
// 否则门户取主域本身、其余站点取「前缀.主域」
func ForRoot(root string) Hosts {
	root = normalizeHost(root)
	if root == "" || root == RootDomain {
		return Hosts{
			Portal:  KindPortal + "." + RootDomain,
			Console: KindConsole + "." + RootDomain,
			API:     KindAPI + "." + RootDomain,
			Img:     KindImg + "." + RootDomain,
			Service: KindService + "." + RootDomain,
		}
	}
	return Hosts{
		Portal:  root,
		Console: KindConsole + "." + root,
		API:     KindAPI + "." + root,
		Img:     KindImg + "." + root,
		Service: KindService + "." + root,
	}
}

// PortalBase 门户站点基地址
func (h Hosts) PortalBase() string { return "https://" + h.Portal }

// ConsoleBase 控制台站点基地址
func (h Hosts) ConsoleBase() string { return "https://" + h.Console }

// APIBase 下游 API 站点基地址
func (h Hosts) APIBase() string { return "https://" + h.API }

// ImgBase 对外图片站点基地址
func (h Hosts) ImgBase() string { return "https://" + h.Img }

// ServiceBase 服务网页承接站点基地址
func (h Hosts) ServiceBase() string { return "https://" + h.Service }

// Split 拆分访问域名：剥离已知站点子域前缀，返回主域与站点类型。
// 未带已知前缀时视为门户（主域即该域名本身）；无法解析时 ok 为 false。
func Split(host string) (root, kind string, ok bool) {
	h := normalizeHost(host)
	if h == "" || !strings.Contains(h, ".") {
		return "", "", false
	}
	parts := strings.Split(h, ".")
	if len(parts) >= 3 && isKnownPrefix(parts[0]) {
		return strings.Join(parts[1:], "."), parts[0], true
	}
	return h, KindPortal, true
}

// RootDomainOf 返回访问域名所属主域（无法解析时返回空串）
func RootDomainOf(host string) string {
	root, _, _ := Split(host)
	return root
}

// IsPlatform 判断访问域名是否属于平台自有域
func IsPlatform(host string) bool {
	root, _, ok := Split(host)
	return ok && root == RootDomain
}

func isKnownPrefix(p string) bool {
	for _, k := range knownPrefixes {
		if p == k {
			return true
		}
	}
	return false
}

// normalizeHost 归一化域名：去协议与路径、端口、末尾点，转小写
func normalizeHost(host string) string {
	h := strings.TrimSpace(strings.ToLower(host))
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	if i := strings.IndexAny(h, "/?#"); i >= 0 {
		h = h[:i]
	}
	if i := strings.Index(h, ":"); i >= 0 {
		h = h[:i]
	}
	return strings.Trim(h, ".")
}
