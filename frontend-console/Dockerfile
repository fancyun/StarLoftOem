# ========================
# Stage 1: 构建门户前端（www.oem.example.com）
# ========================
FROM node:20-alpine AS portal-builder

WORKDIR /app/portal

# npm 镜像（腾讯云内网，避免访问 registry.npmjs.org 超时）
ENV NPM_CONFIG_REGISTRY=https://mirrors.cloud.tencent.com/npm/

# 复制 package 文件
COPY frontend-portal/package*.json ./

# 安装依赖
RUN npm install --prefer-offline --no-audit

# 复制源代码
COPY frontend-portal/ .

# 构建生产版本（增加Node.js内存限制到4GB）
ENV NODE_OPTIONS="--max-old-space-size=4096"
RUN npm run build

# ========================
# Stage 2: 构建控制台前端（console.oem.example.com）
# ========================
FROM node:20-alpine AS console-builder

WORKDIR /app/console

# npm 镜像（腾讯云内网，避免访问 registry.npmjs.org 超时）
ENV NPM_CONFIG_REGISTRY=https://mirrors.cloud.tencent.com/npm/

# 复制 package 文件
COPY frontend-console/package*.json ./

# 安装依赖
RUN npm install --prefer-offline --no-audit

# 复制源代码
COPY frontend-console/ .

# 构建生产版本（增加Node.js内存限制到4GB）
ENV NODE_OPTIONS="--max-old-space-size=4096"
RUN npm run build

# ========================
# Stage 3: 构建管理后台前端（admin.oem.example.com）
# ========================
FROM node:20-alpine AS admin-builder

WORKDIR /app/admin

# npm 镜像（腾讯云内网，避免访问 registry.npmjs.org 超时）
ENV NPM_CONFIG_REGISTRY=https://mirrors.cloud.tencent.com/npm/

# 复制 package 文件
COPY frontend-admin/package*.json ./

# 安装依赖
RUN npm install --prefer-offline --no-audit

# 复制源代码
COPY frontend-admin/ .

# 构建生产版本（增加Node.js内存限制到4GB）
ENV NODE_OPTIONS="--max-old-space-size=4096"
RUN npm run build

# ========================
# Stage 4: 构建服务承接前端（service.oem.example.com）
# ========================
FROM node:20-alpine AS service-builder

WORKDIR /app/service

# npm 镜像（腾讯云内网，避免访问 registry.npmjs.org 超时）
ENV NPM_CONFIG_REGISTRY=https://mirrors.cloud.tencent.com/npm/

# 复制 package 文件
COPY frontend-service/package*.json ./

# 安装依赖
RUN npm install --prefer-offline --no-audit

# 复制源代码
COPY frontend-service/ .

# 构建生产版本（增加Node.js内存限制到4GB）
ENV NODE_OPTIONS="--max-old-space-size=4096"
RUN npm run build

# ========================
# Stage 5: Runtime (Nginx) 多站点
# 同一容器通过不同 server 块按域名分发各前端与后端反代。
# 本容器只监听 3000（HTTP，无证书），容器对外 TLS 由上游反代终止，两种接入方式：
#   A. 平台侧 1Panel 反代各域名到 http://127.0.0.1:3000；
#   B. 3000 直接暴露公网，由部署方自建 HTTPS 反代指向本机 3000（证书/备案归部署方）。
# 两者都只需透传 Host / X-Forwarded-For / X-Forwarded-Proto，无需改写 Host。
#
# 各站点 server_name 采用「平台精确域名 + 子域前缀正则」双写：
# 白标域名（console.example.com 等）按前缀自动落到对应站点块，因此 1Panel 侧无需任何特殊配置。
# Nginx 的 server_name 匹配优先级为「精确 > 通配 > 正则」，故平台自有域始终精确命中、不受正则影响；
# 未带已知前缀的域名（白标门户裸主域等）落到下方 default_server 兜底块。
# 例外：admin 站点只精确匹配 admin.oem.example.com（管理后台不对外开放白标，加正则反而会让
#       任意 admin.* 域名落到后台站点，故刻意不加）。
# ========================
FROM nginx:alpine

# 复制自定义nginx配置
COPY <<'EOF' /etc/nginx/conf.d/default.conf
# 上游反代（1Panel 或部署方自建反代）已终止 TLS，其透传的 X-Forwarded-Proto 优先；
# 未携带时回退本连接协议（本容器只监听 HTTP，即 http）。
map $http_x_forwarded_proto $forwarded_proto {
    default $http_x_forwarded_proto;
    ""      $scheme;
}

# 默认站点（未匹配 server_name 时兜底到门户）；同时承载健康检查
server {
    listen 3000 default_server;
    server_name _;

    location = /healthz {
        return 200 'ok';
    }

    # 兜底：按门户站点渲染
    root /usr/share/nginx/html/www;
    index index.html;
    location / {
        try_files $uri $uri/ /index.html;
    }
}

# 门户站点 www.oem.example.com
server {
    listen 3000;
    server_name www.oem.example.com www ~^www\..+$;
    root /usr/share/nginx/html/www;
    index index.html;

    # Gzip压缩
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/javascript application/xml+rss application/json;

    # 安全头
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # SPA路由支持
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # 公开配置（品牌/客服联系方式/支付渠道）：门户需读取后端公开配置，同样反代到后端
    resolver 127.0.0.11 valid=30s;
    set $backend_endpoint backend:8080;
    location ^~ /console/config {
        proxy_pass http://$backend_endpoint;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
    }

    # API 仅由 api.oem.example.com 承载，本站点拒绝 /api 访问
    location /api/ { return 404; }
    location = /api { return 404; }
}

# API 域 api.oem.example.com：对外下游 API 路径为 /v1/*（用户可见，保持不变），
# 后端一级段取子域标签（/api/v1/*），故此处用 rewrite 映射后再透传。
server {
    listen 3000;
    server_name api.oem.example.com ~^api\..+$;
    server_tokens off;

    # 后端为 compose 服务名，用变量 + resolver 运行时解析，避免容器启动时 DNS 未就绪触发 [emerg] 崩溃
    resolver 127.0.0.11 valid=30s;
    set $backend_endpoint backend:8080;

    # 反代所有请求到后端（后端仅在容器内网暴露）
    location / {
        rewrite ^/v1/(.*)$ /api/v1/$1 break;
        proxy_pass http://$backend_endpoint;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
    }
}

# 图片媒体子域 img.oem.example.com：对外图片地址 /uploads/yyyyMMdd/{md5}.jpg（用户可见，保持不变），
# 后端一级段取子域标签（/img/uploads/*），故此处用 rewrite 映射后再透传（查询串 ?e=&s= 保留）。
server {
    listen 3000;
    server_name img.oem.example.com ~^img\..+$;
    server_tokens off;

    resolver 127.0.0.11 valid=30s;
    set $backend_endpoint backend:8080;

    location /uploads/ {
        # 注意：proxy_pass 同时含变量与 URI 时 nginx 无法拼接剩余路径（URI 被原样发出，路径截断），
        # 因此统一先用 rewrite 改写成后端路径，再用无 URI 的 proxy_pass 原样透传。
        rewrite ^/uploads/(.*)$ /img/uploads/$1 break;
        proxy_pass http://$backend_endpoint;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
    }
}

# 服务承接站点 service.oem.example.com（独立前端项目 frontend-service，承载 FV 承接页等网页流程）
server {
    listen 3000;
    server_name service.oem.example.com ~^service\..+$;
    root /usr/share/nginx/html/service;
    index index.html;

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/javascript application/xml+rss application/json;

    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # 后端为 compose 服务名，用变量 + resolver 运行时解析，避免容器启动时 DNS 未就绪触发 [emerg] 崩溃
    resolver 127.0.0.11 valid=30s;
    set $backend_endpoint backend:8080;

    # 承接页接口（后端一级段取子域标签 /service/*，对外路径一致）：订单状态查询。
    # proxy_pass 含变量时不拼接 URI，故先 rewrite 再透传，避免路径截断。
    location /service/ {
        rewrite ^/service/(.*)$ /service/$1 break;
        proxy_pass http://$backend_endpoint;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
    }

    # 公开配置（品牌/上游核身基址）：承接页需读取后端公开配置，同样反代到后端
    location ^~ /console/config {
        proxy_pass http://$backend_endpoint;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
    }

    # SPA路由支持（FV 承接页 /fv/auth / /fv/self 由服务承接前端渲染）
    location / {
        try_files $uri $uri/ /index.html;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # API 仅由 api.oem.example.com 承载，本站点拒绝 /api 访问
    location /api/ { return 404; }
    location = /api { return 404; }
}

# 控制台站点 console.oem.example.com
server {
    listen 3000;
    server_name console.oem.example.com ~^console\..+$;
    root /usr/share/nginx/html/console;
    index index.html;

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/javascript application/xml+rss application/json;

    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # SPA路由支持
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 控制台 API 代理（控制台与后端同域访问）
    # 用 ^~ 前缀精确匹配，跳过下方静态资源正则 location（~* \.jpg$ 等），
    # 否则 /console/*.jpg 类请求会被静态资源 location 拦截并从本地磁盘找文件 → 404，
    # 而不会反代到后端。^~ 使该前缀匹配优先于所有正则 location。
    resolver 127.0.0.11 valid=30s;
    set $backend_endpoint backend:8080;
    location ^~ /console/ {
        proxy_pass http://$backend_endpoint;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
    }

    # 资质图同域读取：控制台页面按 /img/uploads/* 拉取（带登录态），此处透传到后端 /img/uploads/*
    location ^~ /img/ {
        proxy_pass http://$backend_endpoint;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}

# 管理后台站点 admin.oem.example.com
server {
    listen 3000;
    server_name admin.oem.example.com;
    root /usr/share/nginx/html/admin;
    index index.html;

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/javascript application/xml+rss application/json;

    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # SPA路由支持
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 管理后台 API 代理（管理后台与后端同域访问）
    # 用 ^~ 跳过下方静态资源正则 location，避免 admin 域图片类资源被静态 location 拦走。
    resolver 127.0.0.11 valid=30s;
    set $backend_endpoint backend:8080;
    location ^~ /admin/ {
        proxy_pass http://$backend_endpoint;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
    }

    # 资质图同域读取：管理后台按 /img/uploads/* 拉取（带 admin token），此处透传到后端 /img/uploads/*
    location ^~ /img/ {
        proxy_pass http://$backend_endpoint;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
EOF

# 从builder复制构建产物
COPY --from=portal-builder /app/portal/dist /usr/share/nginx/html/www
COPY --from=console-builder /app/console/dist /usr/share/nginx/html/console
COPY --from=admin-builder /app/admin/dist /usr/share/nginx/html/admin
COPY --from=service-builder /app/service/dist /usr/share/nginx/html/service

# 暴露端口（HTTP，无证书）
EXPOSE 3000

# 覆盖 nginx:alpine 默认 ENTRYPOINT：镜像自带 /docker-entrypoint.sh 会在
# 执行 10-listen-on-ipv6-by-default.sh 等模板脚本时于部分环境卡住，导致 nginx 永不启动
# （容器内 3000 无监听、healthcheck Connection refused）。本镜像的 default.conf 为自管理
# 完整配置，无需镜像 entrypoint 模板，故直接前台启动 nginx。
ENTRYPOINT ["nginx", "-g", "daemon off;"]

# 健康检查（用 127.0.0.1 而非 localhost：容器内 localhost 解析到 ::1 而 nginx 仅监听 IPv4）
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:3000/healthz || exit 1