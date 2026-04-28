# 部署说明

本项目基于 new-api 二次开发，新增了 Discord 身份组校验、可疑用户检测、请求头审计、多次兑换码等功能。

下面提供三种部署方式，按推荐程度排列。

---

## 一、环境要求

| 依赖 | 版本要求 | 说明 |
|---|---|---|
| Go | >= 1.22 | 编译后端 |
| Bun | >= 1.0 | 安装前端依赖 + 构建前端（也可以用 Node.js 18+ 替代） |
| Git | 任意 | 可选，用于版本管理 |
| Docker | >= 20.10 | 仅 Docker 部署需要 |

如果你只想运行已编译好的二进制，只需要操作系统本身即可，不需要 Go 和 Bun。

---

## 二、方式一：Docker 部署（推荐生产环境）

### 1. 构建镜像

在项目根目录执行：

```bash
docker build -t new-api:latest .
```

这会自动完成前端构建、后端编译、打包镜像。

### 2. 使用 docker-compose

项目已提供 `docker-compose.yml`，默认使用 PostgreSQL + Redis。

**修改密码**（生产环境必须改）：

```bash
cp .env.example .env
```

编辑 `.env`，至少设置：

```env
# 会话密钥，必须改为随机字符串
SESSION_SECRET=你的随机字符串

# 端口
PORT=3000

# Discord 校验（如需使用）
DISCORD_REQUIRED_GUILD_ID=你的服务器ID
DISCORD_REQUIRED_ROLE_IDS=角色ID1,角色ID2
```

编辑 `docker-compose.yml`，修改所有 `123456` 为你自己的密码。

**启动：**

```bash
docker-compose up -d
```

**访问：**

```
http://你的服务器IP:3000
```

默认管理员账号：`root` / `123456`，首次登录后请立即修改密码。

### 3. 仅 SQLite（轻量部署）

如果不想装 PostgreSQL 和 Redis，可以修改 `docker-compose.yml`：

- 删除 `postgres`、`redis` 相关段落
- 删除 `SQL_DSN` 和 `REDIS_CONN_STRING` 环境变量
- 添加 `SQLITE_PATH=/data/one-api.db`

数据库文件会保存在 `./data/one-api.db`。

---

## 三、方式二：Linux 服务器源码编译部署

### 1. 安装依赖

**Ubuntu/Debian：**

```bash
# 安装 Go（以 1.22 为例，请根据实际版本调整）
wget https://go.dev/dl/go1.22.4.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 安装 Bun
curl -fsSL https://bun.sh/install | bash
source ~/.bashrc

# 验证
go version
bun --version
```

### 2. 构建前端

```bash
cd web
bun install
bun run build
cd ..
```

构建产物在 `web/dist/`。

### 3. 构建后端

```bash
go build -o new-api .
```

生成可执行文件 `new-api`。

### 4. 配置

```bash
cp .env.example .env
```

编辑 `.env`，最小配置：

```env
PORT=3000
SESSION_SECRET=一个随机字符串
SQLITE_PATH=./one-api.db
```

如果使用 MySQL 或 PostgreSQL，设置 `SQL_DSN` 替代 `SQLITE_PATH`：

```env
# PostgreSQL
SQL_DSN=postgresql://user:password@localhost:5432/new-api

# MySQL
SQL_DSN=user:password@tcp(127.0.0.1:3306)/new-api?parseTime=true
```

Discord 校验（可选）：

```env
DISCORD_REQUIRED_GUILD_ID=你的服务器ID
DISCORD_REQUIRED_ROLE_IDS=角色ID1,角色ID2
```

### 5. 运行

```bash
chmod +x new-api
./new-api --port 3000 --log-dir ./logs
```

### 6. 设为系统服务（推荐）

项目提供了 `new-api.service` 模板，复制并修改：

```bash
sudo cp new-api.service /etc/systemd/system/new-api.service
sudo nano /etc/systemd/system/new-api.service
```

修改其中的用户名、路径、端口，然后：

```bash
sudo systemctl daemon-reload
sudo systemctl enable new-api
sudo systemctl start new-api
sudo systemctl status new-api
```

---

## 四、方式三：Windows 本地源码开发/部署

### 1. 安装依赖

- 安装 Go：https://go.dev/dl/
- 安装 Bun：https://bun.sh/ 或用 `npm install -g bun`
- 确保 `go` 和 `bun` 在 PATH 中

### 2. 配置环境

编辑 `local-env.ps1`，将路径改为你本机的 Go 和 Bun 安装路径：

```powershell
$env:Path = "你的Go路径\bin;你的Bun路径;$env:Path"
$env:GOPROXY = 'https://goproxy.cn,direct'  # 国内用户建议保留
```

### 3. 安装前端依赖

```powershell
.\install-frontend.ps1
```

### 4. 构建前端

```powershell
.\build-frontend.ps1
```

### 5. 配置 .env

在项目根目录创建 `.env`：

```env
PORT=3000
SQLITE_PATH=./one-api.db
SESSION_SECRET=一个随机字符串
```

### 6. 构建并启动

```powershell
.\build-backend.ps1
.\start-local.ps1
```

访问 `http://localhost:3000`。

### 7. 停止

```powershell
.\stop-local.ps1
```

---

## 五、首次使用

1. 访问 `http://你的地址:3000`。
2. 默认管理员账号：`root`，密码：`123456`。
3. **立即修改密码。**
4. 进入「系统设置」配置：
   - Discord OAuth（Client ID、Client Secret、服务器 ID、身份组 ID）
   - 模型渠道
   - 限速规则
5. 进入「兑换码管理」创建福利兑换码：
   - 设置「每个兑换码可用次数」为你需要的值（如 100）
   - 每个用户对同一兑换码只能使用一次
6. 进入「可疑用户」页面查看异常使用行为

---

## 六、API 调用地址

客户端（SillyTavern、ChatBox 等）填写：

```
API Base URL: http://你的地址:3000/v1
API Key:      你创建的令牌（sk-xxxx）
```

常用接口：

| 接口 | 路径 |
|---|---|
| 模型列表 | `GET /v1/models` |
| 聊天补全 | `POST /v1/chat/completions` |
| 文本补全 | `POST /v1/completions` |

---

## 七、反向代理（可选）

如果用 Nginx 反向代理，参考配置：

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_cache off;
        chunked_transfer_encoding on;
        tcp_nopush on;
        tcp_nodelay on;
        keepalive_timeout 300;
    }
}
```

如需 HTTPS，建议用 Certbot 自动签发证书。

---

## 八、项目自定义功能说明

本项目在 new-api 基础上增加了以下功能：

### Discord 身份校验

- `DISCORD_REQUIRED_GUILD_ID`：**必填**，用户必须在该 Discord 服务器中
- `DISCORD_REQUIRED_ROLE_IDS`：可选，多个角色 ID 用逗号分隔，拥有任意一个即可通过
- 也可以在后台「系统设置 > Discord OAuth」中配置

### 请求头审计

- 超级管理员在使用日志中可以查看每条请求的客户端信息和请求头
- 自动识别 SillyTavern、ChatBox、newapi 等客户端
- 普通用户和普通管理员看不到请求头信息
- 敏感头（Authorization、Cookie 等）不会被记录

### 可疑用户检测

- 后台「可疑用户」页面，基于消费日志实时聚合
- 检测规则：单用户 IP 超过 3 个、全天候使用、频繁接近限速、请求量异常
- 每个可疑用户会列出具体原因和风险评分
- 仅用于辅助人工审判

### 多次兑换码

- 创建兑换码时可设置「每个兑换码可用次数」
- 同一用户对同一兑换码只能使用一次
- 列表显示已用次数/总次数
- 适合发放社区福利

---

## 九、常见问题

**Q：启动报 `SESSION_SECRET is set to the default value`？**

A：必须在 `.env` 中设置 `SESSION_SECRET` 为一个随机字符串，不能是 `random_string`。

**Q：前端构建报内存不足？**

A：设置环境变量 `NODE_OPTIONS=--max-old-space-size=4096`。

**Q：SQLite 报 `database is locked`？**

A：在 `SQLITE_PATH` 后面加 `?_busy_timeout=30000`，例如：
```env
SQLITE_PATH=./one-api.db?_busy_timeout=30000
```

**Q：国内 `go mod download` 很慢？**

A：设置 Go 代理：
```bash
export GOPROXY=https://goproxy.cn,direct
```

**Q：客户端获取不到模型？**

A：检查以下几点：
1. Base URL 应该填 `http://地址:3000/v1`，不是 `/v1/models`
2. 必须带 `Authorization: Bearer sk-你的令牌`
3. 令牌所属分组必须有可用渠道和模型
4. 如果在其他设备上访问，不能用 `127.0.0.1`，要用服务器实际 IP
