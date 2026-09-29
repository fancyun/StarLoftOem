# StarLoft 星楼网络 · 综合云服务平台

[![Version](https://img.shields.io/badge/version-v1.25.2-blue.svg)](https://github.com/fancyun/StarLoft)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.20+-00ADD8.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/docker-20.10+-2496ED.svg)](https://www.docker.com/)

**星楼网络** 是一个综合云服务平台（对标腾讯云 / 阿里云架构）：平台门户 + 独立产品（人脸核验、短信服务等）+ 统一控制台与账户系统。当前已上线**人脸核验（FV）**产品（有源 fv_auth / 无源 fv_self）与**短信服务（SMS）**，提供 API 调用、资源包计费与 PHP 插件对接。

---

## 🎯 核心特性

- 🚀 **综合云平台**: 门户按 `/product/{key}` 聚合产品入口，产品独立分库（一产品一库），横向可扩展新云产品
- 🔑 **API 安全**: API Key/Secret 双重鉴权 + HMAC-SHA256 签名，密钥可按下游端点逐个授权（`all` 或逗号分隔的端点标识）
- 🔌 **易于集成**: RESTful API + PHP 插件（智简魔方财务版 / 业务系统 v10，人脸核验 FV）
- 📊 **完善管理**: 后台 Dashboard + 用户/订单/资源包/日志管理
- 💰 **灵活计费**: 资源包优先、余额兜底；购买支持余额 + 在线支付组合支付
- 📣 **推广分佣**: 用户打开控制台推广页即自动开通推广码（`?ref=` 归属），按下级**利润**（实付 − 成本 × 件数）自动计提提成，人脸核验 / 短信可分别设置比例与成本；用户型推广仅归因被推广用户注册后 1 个自然月内的消费、收益可提现到平台余额
- 🧑💼 **后台多账号**: 管理员与员工同为后台账号，创建时按功能逐项勾选权限（无角色列）；员工自动成为销售，提供「员工管理」与「销售业绩（按月）」页面
- 🎨 **现代UI**: 腾讯云官网设计风格，门户与控制台均适配手机 / 平板 / 电脑
- 🧩 **人脸核验**: 实名认证（个人/企业）+ 人脸识别（有源/无源）完整流程
- 🗃️ **日志落盘**: 控制台/管理后台/API 访问与第三方调用回调全部写入 `/data/logs` 分文件记录，便于审计

---

## 🏗️ 平台架构

```
www.starloft.cn      门户站点（frontend-portal / frontend-service）：平台首页、产品页、文档中心、FV承接页
console.starloft.cn  控制台（frontend-console）：登录注册、实名认证、资源包、充值、API 管理
admin.starloft.cn    管理后台（frontend-admin）：数据统计、用户/订单/资源包管理
service.starloft.cn  服务承接页（frontend-service）：人脸核验承接页（/fv/auth、/fv/self）
api.starloft.cn      API 域（纯 Nginx 反代，无前端）：对外下游服务接口 /v1/*
```

- 门户聚合产品入口，产品页路径为 `/product/fv`、`/product/sms`
- 门户、控制台共用同一账户体系；登录 / 注册 / 实名认证均位于控制台
- 前端共用一个**本地化 Nginx 多站点容器**，监听 `3000`（HTTP，无证书），含内置`serve`静态托管与 SPA 回退，按 `server_name` 分发 `www/console/admin/service` 并反代 `api` 至后端
- **后端仅在容器内网暴露 `8080`**，由本地 Nginx 反代；后端路由一级段取访问子域标签（`/console/*`、`/admin/*`、`/img/uploads/*`、`/api/v1/*`），用户可见 URL（`api.starloft.cn/v1/*`、`img.starloft.cn/uploads/*`）保持不变
- 对外 TLS/证书由 1Panel（宿主机 Nginx）反代到 `http://127.0.0.1:3000` 时统一配置，需透传 `Host` 头
- 旧域名 `kyc.starloft.cn` 80 端口自动重定向至门户（由 1Panel/外部 Nginx 配置）

---

## 🚀 快速开始

### Docker 部署（推荐）

> 说明：MySQL、Redis、后端、前端（Nginx 多站点）全部由 docker-compose 编排。MySQL 数据持久化到项目 `./data/mysql`，日志落具名卷 `logs_data`。前端 Nginx 监听 `3000`（HTTP，无证书），对外 TLS/证书由 1Panel（宿主机 Nginx）反代时配置。

```bash
# 1. 克隆代码
git clone https://github.com/fancyun/StarLoft.git
cd StarLoft

# 2. 配置环境变量
cp .env.example .env
nano .env   # 填写数据库(root密码)/Redis/JWT/加密密钥及第三方密钥等自举与密钥类配置
#             非密钥类业务配置（单价/开关/AppID/地域等）不在本文件，只存数据库，启动后在后台「系统设置 / 产品配置」维护

# 3. 启动服务（建库/建表/种子数据由后端启动时自动完成，无需手动执行 init.sql）
#    首次启动前确保宿主机已准备 ./data/mysql 目录（MySQL 数据目录，勿放 NFS）
docker compose up -d --build
```

服务端口：
- 前端/API 入口：`http://127.0.0.1:3000`（按 `server_name` 分发门户/控制台/管理后台/服务承接页，并反代 `api` 至后端）
- 后端：容器内网 `8080`（仅由前端 Nginx 反代，不直接暴露宿主）
- 反向代理：在 1Panel/宿主机 Nginx 中将各域名反代到 `http://127.0.0.1:3000` 并配置 Let's Encrypt 证书（透传 `Host` 头以按域名分发）。

> 注意：`docker compose` 仅提供 HTTP 入口（3000）；对外 HTTPS/证书需在 1Panel/宿主机 Nginx 完成。

初始化说明：
- 分库架构：系统库 `starloft_sys` 由 MySQL 容器 `MYSQL_DATABASE` 创建；其余分库（`starloft_fv`、`starloft_sms`）由后端迁移自动创建。
- 表结构由后端启动时 GORM **AutoMigrate** 自动补齐；默认管理员与默认演示用户由后端启动时自动写入（幂等，不再种子资源包）。
- 已不再使用 `database/init.sql`，无需手动初始化。

访问系统：

| 站点 | 地址 |
| --- | --- |
| 门户 | https://www.starloft.cn |
| 控制台 | https://console.starloft.cn |
| 管理后台 | https://admin.starloft.cn |
| 服务承接页 | https://service.starloft.cn/fv/auth |
| API | https://api.starloft.cn/v1 |
| 文档 | https://www.starloft.cn/docs |

### 新增一个云产品

1. 在 `frontend-portal/src/config/products.ts` 登记产品信息（key / 名称 / 特性 / 场景 / 控制台地址）
2. 在 `frontend-portal/src/router/index.ts` 注册产品路由（复用 `ProductPage.vue`，`meta.product` 指定 key）
3. 在 `frontend-console/src/router/index.ts` 为控制台增加产品路由与侧边栏入口
4. 按"一产品一库"新增独立库与订单表（从 `000003` 起新增迁移创建库/表），并在 `internal/model/database.go` 登记库名与 AutoMigrate
5. 重新构建前端与后端镜像后上线

---

## 📚 文档中心

- **[数据库架构文档](docs/database.md)** - 🗃️ 分库设计 / 各表字段说明
- **API v1 文档** - 🔌 鉴权方式、签名算法、接口说明（Portal [在线版](https://www.starloft.cn/docs/fv/api/v1)）
- **联麓短信 API** - 📨 [docs/shlianlu_sms_api.md](docs/shlianlu_sms_api.md)
- **Yunjinzhi 上游 API** - 📄 [docs/ylzj_fv_api.md](docs/ylzj_fv_api.md)
- **PHP 插件（人脸核验 / 短信）** - 🔌 [StarLoftPlugin](https://github.com/fancyun/StarLoftPlugin)（智简魔方财务版 / 业务系统 v10，独立仓库）

---

## 🏗️ 技术架构

```
后端: Go 1.20+ + Gin
数据库: MySQL 8.0（docker-compose 本地容器，分库架构，数据落 ./data/mysql）
缓存: Redis 7.0
前端: Vue 3 + Vite（frontend-portal / frontend-console / frontend-admin / frontend-service）
部署: Docker + Docker Compose；前端本地化 Nginx 监听 3000（HTTP）；对外 TLS/反代由 1Panel（宿主机 Nginx 统一管理）
身份核验: 个人/企业实名走 FinAuth 人证核验；平台企业实名法人扫脸走腾讯云人脸核身（慧眼）
短信: 下游短信产品走联麓（shlianlu）短信网关（仅模板发送，验证码/通知与营销分通道、分资源包）；平台验证码短信走腾讯云短信
支付: 支付宝（电脑网站支付）
验证码: 腾讯云天御（人机验证）
```

---

## 📦 项目结构

```
StarLoft/
├── README.md               # 项目入口（本文件）
├── docs/                   # 文档
│   ├── database.md         # 数据库架构说明文档
│   ├── shlianlu_sms_api.md # 联麓短信 API 文档（现行下游短信通道）
│   └── ylzj_fv_api.md      # 上游(云今至) API 文档
├── docker-compose.yml      # Docker 编排（mysql/redis/backend/frontend Nginx 多站点）
├── .env.example            # 环境变量模板（仅密钥类 + 基础设施自举参数；非密钥业务配置只存数据库）
├── certs/                  # SSL 证书（Let's Encrypt 泛域名 fullchain.pem/privkey.pem，gitignore；由 1Panel/外部 Nginx 使用）
├── backend/                # Go 后端服务
│   └── internal/
│       ├── handler/        # HTTP 处理器
│       ├── service/        # 业务逻辑
│       ├── repository/     # 数据访问
│       ├── model/          # 数据模型
│       ├── router/         # 路由（/v1 /console /admin）
│       ├── middleware/     # 鉴权 / 访问控制 / 请求日志中间件
│       ├── logstore/       # 日志分类落盘（日志入库已废弃）
│       ├── upstream/       # 上游 FinAuth / 支付宝 / 联麓短信客户端
│       └── cron/           # 定时任务（每日对账等）
├── frontend-console/       # 控制台前端（console.starloft.cn）
├── frontend-admin/         # 管理后台前端（admin.starloft.cn）
├── frontend-portal/        # 门户前端（www.starloft.cn）
└── frontend-service/       # 服务承接前端（service.starloft.cn，FV 承接页）
```

> 注：PHP 插件与 SDK 已迁至**独立仓库**，本仓库 `.gitignore` 忽略、本地无对应目录：
> - 插件：<https://github.com/fancyun/StarLoftPlugin>（智简魔方财务版 / 业务系统 v10，人脸核验 FV + 短信）
> - SDK：<https://github.com/fancyun/StarLoftSdk>

---

## 🚀 主要功能

### 平台门户（www.starloft.cn + service.starloft.cn）
- ✅ 平台首页（产品聚合、平台优势）
- ✅ 产品页：人脸核验（/product/fv）、短信服务（/product/sms）
- ✅ 文档中心（按产品分层 /fv|sms 下挂 docs、api、plugin、pricing）：API v1 文档、插件教程、定价
- ✅ FV 人脸核验承接页（service.starloft.cn/fv/auth|self，PC 扫码 / 移动端内嵌上游核身）

### 用户功能（console.starloft.cn）
- ✅ 手机号注册登录
- ✅ 账户实名认证（Web 端免费；个人实名 + 企业实名，实名成功后信息永久绑定）
- ✅ 身份核验 API 调用（个人/企业；余额 + 资源包双计费）
- ✅ 资源包购买（支持余额 + 在线支付组合支付；待支付订单 30 分钟未支付自动关闭，已扣余额部分原路退还）
- ✅ 在线充值（支付宝）
- ✅ 余额查询、消费记录
- ✅ API 密钥管理（实名后自行创建 Key/Secret，一个账号可建多把，各自按端点授权「全部接口」或指定若干接口）
- ✅ 推广中心（打开即自动开通推广码、推广码与推广链接、提成收益明细、推广用户、提现到余额；提成按利润计提，仅归因被推广用户注册后 1 个自然月内的消费）

### 管理后台（admin.starloft.cn）
- ✅ 账号权限体系（管理员 / 员工同为后台账号，按模块勾选「查看」与「修改」两档，另可勾「全部权限」或某分区的「全部（读写）」；超管即持有全部权限的账号，不依赖账号 ID；未分配权限的账号无法进入后台）
- ✅ 数据统计 Dashboard
- ✅ 用户管理（搜索 / 详情 / 状态管理 / 账户实名两档的定向定价 / 重置实名次数 / 人工充值）
- ✅ 认证记录管理（记录详情 / 失败原因 / 手动查询上游结果）
- ✅ 资源包管理（创建 / 上下架 / 产品与价格 / 向指定用户发放测试包；人脸核验按有源、无源分开发放）
- ✅ 产品配置（各产品页维护单价与成本单价，并可对单个用户设置定向定价，覆盖平台价）
- ✅ 人工企业实名（录入企业名称 + 统一社会信用代码）
- ✅ 推广管理（提成记录、提现审核）
- ✅ 员工管理（新增/编辑账号、启用停用、重置密码；创建即自动获得销售推广身份）
- ✅ 销售业绩（按月查看：本月与累计提成、推广码与推广链接、提成流水、推广客户）
- ✅ 系统设置（**非密钥类**配置在线维护：产品单价与成本单价、开关/AppID/地域、支付风控阈值、推广提成比例、客服联系方式（门户首页实时展示）；密钥类配置只保存在 `.env`，不入库）

### 开发者功能
- ✅ RESTful API（API Key + HMAC-SHA256 签名）
- ✅ Webhook 回调通知（notify_url）+ 服务承接回跳
- ✅ PHP 插件（智简魔方财务系统 / v10，人脸核验 FV）

---

## 🔐 安全特性

- ✅ API 密钥最小长度校验 + JWT 密钥强度校验（不达标拒绝启动）
- ✅ API Key/Secret + HMAC-SHA256 请求签名（防篡改 / 防重放），权限按下游端点隔离，并叠加实名等级门槛（人脸核验需企业实名、短信需个人实名）
- ✅ Redis 口令保护，仅容器内网访问、不映射公网端口
- ✅ 后端不暴露公网端口，仅经 Nginx TLS 反代
- ✅ SQL 注入防护、API 限流保护
- ✅ 敏感数据 AES-256-GCM 加密、密码 bcrypt 存储
- ✅ 数据库 TLS 连接、日志电话号/IP 脱敏
- ✅ 健康探针（/healthz、/readyz）与 Prometheus 指标（/metrics，仅内网）
- ✅ 全量操作日志落盘（控制台 / 管理后台 / API / 第三方调用），不自动删除

---

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

---

## 📞 技术支持

- 📧 邮箱: support@starloft.tech
- 📖 文档: [在线文档中心](https://www.starloft.cn/docs)
- 🐛 问题反馈: [GitHub Issues](https://github.com/fancyun/StarLoft/issues)

---

**版本**: v1.25.2
**更新日期**: 2026-09-29
**开发团队**: StarLoft Tech Team