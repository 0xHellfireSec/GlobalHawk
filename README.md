# 全球鹰 GlobalHawk (Go + Wails)

跨平台网络空间测绘聚合工具——聚合 FOFA / 鹰图(Hunter) / Shodan / Quake / Zoomeye / Censys
六大引擎的资产搜索、表格化管理与 CSV 导出。

> 当前版本：v1.0.0 ｜ Author：**0xHellfireSec** ｜ 本目录构建目标：macOS arm64 (Apple Silicon)

## 功能

| 功能 | 说明 |
|---|---|
| 六引擎查询 | FOFA / Hunter / Shodan / Quake / Zoomeye / Censys 统一界面 |
| 全量拉取 | 各引擎支持「全部（自动翻页）」模式，一次查询拉取全部结果，自动翻页、防频控、上限保护、中断保留已有数据 |
| 查询量与显示量分离 | 本地分页展示（每页 20/50/100/全部），翻页与调整显示条数不消耗 API 配额 |
| 结果排序 | 点击表头按任意列排序，升/降切换，数字感知，空值沉底 |
| 结果去重 | 按 IP / 按 IP+端口 一键去重，序号保持原始编号 |
| URL 直达 | 结果中的 URL 点击后用系统默认浏览器打开 |
| 搜索历史 | 点击输入框弹出历史关键词（每引擎 30 条，持久化） |
| Quake 语法归一 | 裸域名/裸 IP 自动转 `domain:"..."` / `ip:"..."` |
| URL 列 | Quake/ZoomEye 结果自动拼接可访问 URL（非 web 服务留空） |
| 配额显示 | Quake/ZoomEye/Hunter 剩余积分实时刷新 |
| Censys 游标翻页 | Hosts 搜索支持上一页/下一页 |
| 导出 CSV | 系统原生保存对话框，UTF-8 BOM（Excel 中文不乱码） |
| 语法参考 | 六引擎语法速查（表格化展示） |

## 引擎适配说明

- FOFA 仅校验 key，email 可为空；接口单次最多返回 1 万条
- Hunter 需浏览器 User-Agent 与规范化 URL 编码；积分字段为带标签字符串
- Quake 的 hostname 字段为 rDNS，取值回退 domain → http.host
- ZoomEye 每页固定 20 条，自动翻页模式下按页轮询

## 目录结构

```
HellfireSec/
├── main.go            # Wails 入口
├── app.go             # 绑定到前端的方法层
├── engines/           # 六引擎客户端 + 配置/历史/CSV 公共库
│   ├── engines.go     #   HTTP 客户端、INI 读写、历史存储、CSV
│   ├── types.go       #   统一返回模型 SearchResult
│   ├── fofa.go hunter.go shodan.go quake.go zoomeye.go censys.go
├── frontend/dist/     # 前端（原生 HTML/CSS/JS，无 Node 依赖）
│   ├── index.html
│   └── src/  style.css  app.js  syntax.js
├── build/             # appicon.png / app.icns / 产物 GlobalHawk.app
├── build.sh           # macOS arm64 一键构建脚本
└── PkgInfo
```

## 构建与运行

```bash
./build.sh          # 编译 → 组装 GlobalHawk.app → 临时签名
open build/GlobalHawk.app
```

依赖：Go ≥ 1.21、Xcode Command Line Tools。
国内网络建议先 `go env -w GOPROXY=https://goproxy.cn,direct`；
若链接报 `UTType` 缺失，确认 build.sh 中已带
`CGO_LDFLAGS="-framework UniformTypeIdentifiers"`。

产物单可执行文件约 9MB。

## 数据文件

| 文件 | 用途 |
|---|---|
| `~/config.ini` | 各引擎 API Key |
| `~/globalhawk_history.json` | 搜索历史 + 上次导出目录 |

## 致谢与许可

- 本项目功能设计参考 [G3et/Search_Viewer](https://github.com/G3et/Search_Viewer)（MIT License, Copyright (c) 2023 G3et），
  依据 MIT 协议进行二次实现；语法参考页内容提取自该项目。
- 本项目同样以 MIT 协议发布。Author：0xHellfireSec
