# Cloudflare IP 优选域名管理系统 - 规则系统文档

## 项目概述

cf-optimizer 是一个自动化的 Cloudflare IP 优选系统，通过路由追踪、延迟测试、VLESS 可用性验证和 DNS 自动更新，为不同域名选择经过特定地理位置的最优 Cloudflare IP。

---

## 一、groupRules 配置规则

### 规则定义

配置文件位置：`config.yaml`

```yaml
groupRules:
  SG_GD: [["新加坡"], ["广东"]]
  SG_SH: [["新加坡"], ["上海"]]
  JP_GD: [["日本", "东京都"], ["广东"]]
  JP_SH: [["日本", "东京都"], ["上海"]]
```

### 规则结构说明

- **外层键**：分组名称（如 SG_GD、SG_SH）
- **内层数组**：AND 条件组（必须同时满足）
- **最内层数组**：OR 条件（满足其中任意一个即可）

### 分组规则详解

| 分组名称 | 条件1 (AND) | 条件2 (AND) | 含义 |
|---------|------------|------------|------|
| SG_GD | 必须包含"新加坡" | 必须包含"广东" | 路由经过新加坡和广东 |
| SG_SH | 必须包含"新加坡" | 必须包含"上海" | 路由经过新加坡和上海 |
| JP_GD | 必须包含"日本"或"东京都" | 必须包含"广东" | 路由经过日本/东京都和广东 |
| JP_SH | 必须包含"日本"或"东京都" | 必须包含"上海" | 路由经过日本/东京都和上海 |

### 规则匹配逻辑

以 `JP_GD: [["日本", "东京都"], ["广东"]]` 为例：

- 路由路径中必须同时包含：
  - "日本" **或** "东京都"（满足其中一个即可）
  - **且** "广东"（必须包含）

---

## 二、路由追踪和分组逻辑

### 核心函数

函数位置：`tracer/tracer.go` - `GetIPGroup()`

### 路由追踪流程

```
输入 IP 地址
    ↓
执行 nexttrace 命令
    ↓
解析 JSON 输出
    ↓
提取地理位置信息
    ↓
构建路由路径
    ↓
匹配 groupRules
    ↓
返回分组名称
```

### 地理位置提取规则

#### 优先级顺序

1. **省份信息**（`hop.Geo.Prov`）
   - 提取后去除后缀：省、市、自治区、特别行政区
   - 例如："广东省" → "广东"，"上海市" → "上海"

2. **国家信息**（`hop.Geo.Country`）
   - 仅当省份为空时使用
   - 排除 "Anycast"（任播地址）

#### 代码实现

```go
if hop.Geo.Prov != "" {
    p := hop.Geo.Prov
    p = strings.TrimSuffix(p, "省")
    p = strings.TrimSuffix(p, "市")
    p = strings.TrimSuffix(p, "自治区")
    p = strings.TrimSuffix(p, "特别行政区")
    currentLocation = p
} else if hop.Geo.Country != "" && hop.Geo.Country != "Anycast" {
    currentLocation = hop.Geo.Country
}
```

### 路由路径构建规则

#### 去重逻辑

- 记录 `lastLocation` 避免重复
- 使用 `locationsSet` 存储所有经过的位置

#### 示例路由路径

```
原始路由：上海 → 广东 → 新加坡 → 新加坡
去重后：["上海", "广东", "新加坡"]
locationsSet: {"上海", "广东", "新加坡"}
```

### 分组匹配算法

#### 匹配逻辑

```go
for groupName, andConditions := range config.Current.GroupRules {
    match := true
    for _, orConditions := range andConditions {
        orMatch := false
        for _, location := range orConditions {
            if _, found := locationsSet[location]; found {
                orMatch = true
                break
            }
        }
        if !orMatch {
            match = false
            break
        }
    }
    if match {
        return groupName
    }
}
```

#### 匹配步骤

1. 遍历每个分组规则
2. 对每个 AND 条件组进行检查
3. 在 OR 条件数组中查找匹配项
4. 所有 AND 条件都满足时返回该分组名

#### 匹配示例

假设路由路径为：`["上海", "广东", "新加坡"]`

| 分组 | 条件1检查 | 条件2检查 | 结果 |
|-----|---------|---------|------|
| SG_GD | "新加坡" ✓ | "广东" ✓ | **匹配成功** |
| SG_SH | "新加坡" ✓ | "上海" ✓ | **匹配成功** |
| JP_GD | "日本"/"东京都" ✗ | - | 匹配失败 |
| JP_SH | "日本"/"东京都" ✗ | - | 匹配失败 |

**注意**：如果多个规则都匹配，返回第一个匹配的分组。

---

## 三、域名映射规则

### hostMap 配置

配置文件位置：`config.yaml`

```yaml
hostMap:
  "jp.yx.meiyoukaoshang.dpdns.org":
    group: "JP_SH"
    id: "1960867935af9bb7d8e0fade02aa84d3"
  "sg.yx.meiyoukaoshang.dpdns.org":
    group: "SG_GD"
    id: "b12e42f63f6b7d11d7ad374facb18b84"
  "us.yx.meiyoukaoshang.dpdns.org":
    group: "JP_SH"
    id: "b2e330327cbd3d58a26114b27a6c611e"
```

### 映射关系

| 域名 | 使用分组 | DNS 记录 ID | 说明 |
|-----|---------|------------|------|
| jp.yx.meiyoukaoshang.dpdns.org | JP_SH | 1960867935af9bb7d8e0fade02aa84d3 | 日本-上海路由 |
| sg.yx.meiyoukaoshang.dpdns.org | SG_GD | b12e42f63f6b7d11d7ad374facb18b84 | 新加坡-广东路由 |
| us.yx.meiyoukaoshang.dpdns.org | JP_SH | b2e330327cbd3d58a26114b27a6c611e | 日本-上海路由 |

### 工作原理

1. 系统为每个分组选择最优 IP
2. 根据域名映射，将最优 IP 更新到对应的 DNS 记录
3. 多个域名可以使用同一个分组（如 jp 和 us 都使用 JP_SH）

---

## 四、完整工作流程

### 1. IP 获取阶段

```
从数据源获取 IP（精简：仅 uouin；完整：uouin + ipdb + 智选网）
    ↓
精简模式：直接使用源 IP
完整模式：直接使用源 IP
    ↓
去重处理
    ↓
过滤数据库中已存在的 IP
    ↓
只对新 IP 执行 nexttrace
```

### 2. IP 分组阶段（仅新 IP）

```
对每个新 IP 执行 nexttrace
    ↓
解析路由路径
    ↓
提取地理位置信息
    ↓
匹配 groupRules
    ↓
确定分组名称
    ↓
存入数据库
```

### 3. 延迟测试阶段

对所有候选 IP（含已入库的旧 IP）统一进行延迟测试：

```
查询每个 IP 的分组（新 IP 刚入库，旧 IP 从数据库读取）
    ↓
精简模式执行 2 次 ping，完整模式执行 3 次 ping（间隔 1 秒）
    ↓
计算平均延迟
    ↓
按分组聚合，组内按延迟升序排序
```

### 4. VLESS 验证阶段（可选）

通过启动临时 xray 实例验证 IP 是否真正可用于 VLESS 代理。解决了"ping 通但无法用于 VLESS"的问题。

```
每组取延迟排名前 3 的候选 IP
    ↓
用优选 IP 替换 VLESS URL 中的地址
    ↓
生成 xray 配置（ws + tls + vless）
    ↓
启动 xray 实例监听本地随机端口
    ↓
通过代理访问 verifyURL（默认 http://cp.cloudflare.com/generate_204）
    ↓
返回 200/204 → 验证通过，使用该 IP
返回其他状态/连接失败 → 验证失败，回退到下一个候选 IP
    ↓
所有候选 IP 均失败 → 跳过该分组的 DNS 更新
```

**配置说明**：
- `vless` 留空 → 禁用验证，直接使用延迟最低的 IP
- `vless` 配置后 → 启用验证，启动时会检查 `./xray` 是否存在，缺失即退出

### 5. DNS 更新阶段

```
根据 hostMap 确定域名-分组关系
    ↓
获取每个分组通过验证的最优 IP
    ↓
调用 Cloudflare API
    ↓
更新 DNS A 记录
```

---

## 五、实际应用示例

### 场景：用户访问 sg.yx.meiyoukaoshang.dpdns.org

1. **系统查找**：该域名使用 `SG_GD` 分组
2. **IP 选择**：从内存缓存 `bestIPsByGroup` 获取 SG_GD 分组最近一次优选出的 IP（经 nexttrace 分组、ping 测速、xray 验证）
3. **DNS 解析**：系统已将该 IP 通过 Cloudflare API 更新到 DNS A 记录，用户被解析到该 IP
4. **访问路径**：用户 → 广东 → 新加坡 → Cloudflare

> `/gethosts` 端点返回的就是内存缓存中各分组当前的最优 IP，不实时查库或测速。

### 优势

- 确保路由经过指定地理位置
- 自动选择延迟最低的 IP
- 定时更新保证最优性能
- 支持多域名灵活配置

---

## 六、错误处理规则

### 分组匹配失败时的返回值

| 返回值 | 含义 |
|-------|------|
| UNKNOWN_ERROR | nexttrace 命令执行失败 |
| NO_JSON | nexttrace 输出中未找到 JSON |
| JSON_PARSE_ERROR | JSON 解析失败 |
| UNKNOWN | 路由路径为空 |
| 路径_路径 | 未匹配任何规则，返回原始路径（如 "广东_新加坡"） |

---

## 七、配置扩展指南

### 添加新分组规则

```yaml
groupRules:
  # 现有规则...
  HK_SZ: [["香港"], ["深圳"]]  # 香港-深圳路由
  US_BJ: [["美国"], ["北京"]]  # 美国-北京路由
```

### 添加新域名映射

```yaml
hostMap:
  "hk.example.com":
    group: "HK_SZ"
    id: "your_dns_record_id"
```

### 启用 VLESS 验证

在 `config.yaml` 中配置以下字段即可启用：

```yaml
# VLESS 分享链接（从客户端导出）
vless: "vless://<uuid>@<地址>:<端口>?encryption=none&security=tls&type=ws&host=<域名>&path=<路径>#<别名>"
# 验证 URL，通过代理访问此地址测试 IP 可用性（留空则使用默认值）
verifyURL: "http://cp.cloudflare.com/generate_204"
```

- `vless` 留空 → 禁用验证，仅使用 ping 延迟优选
- `vless` 配置后 → 启用验证，验证失败的 IP 会自动回退到延迟排名下一个候选
- 所有候选 IP 均验证失败 → 跳过该分组的 DNS 更新（保留原记录不变）

### 注意事项

- DNS 记录 ID 需要从 Cloudflare 控制台获取
- 地理位置名称需与 nexttrace 返回的名称一致
- 建议先测试路由路径再配置规则
- 启用 VLESS 验证需要 `./xray` 二进制存在且可执行

---

## 八、运行模式对比

两种模式使用统一的处理流水线（nexttrace → ping → 分组排序 → xray 验证前 3 → DNS 更新），区别在数据源范围、Ping 次数和更新频率：

### 精简模式

- **数据源**：仅 uouin.com（CTCC）
- **IP 处理**：直接使用源 IP
- **延迟测试**：每个 IP 测试 2 次（间隔 1 秒）
- **验证候选**：每组延迟排名前 3
- **更新频率**：每 2 小时
- **适用场景**：快速部署、低资源环境

### 完整模式

- **数据源**：uouin.com + ipdb + 智选网（三源去重）
- **IP 扩展**：无（直接使用源 IP）
- **延迟测试**：每个 IP 测试 3 次（间隔 1 秒）
- **验证候选**：每组延迟排名前 3
- **更新频率**：每 1 小时
- **适用场景**：需要更广 IP 覆盖

---

## 九、核心模块说明

### modes/

- `minimal.go`：精简模式入口 + 共享处理流水线（`processIPs`/`measureIPs`/`updateDNS`/`verifyCandidates`）
- `full.go`：完整模式入口（并发获取三源 IP 后调用共享流水线）
- `handlers.go`：HTTP `/gethosts` 端点，返回最近一次优选结果

### database/database.go

- 使用 SQLite 存储 IP 数据
- 提供插入、按 IP 查分组、过滤已存在 IP 功能

### tracer/tracer.go

- 使用 nexttrace 工具进行路由追踪
- 解析路由路径中的地理位置信息
- 根据配置规则对 IP 进行分组

### latency/latency.go

- 使用 ping 命令测试连接延迟
- 解析 round-trip/rtt 平均值获取延迟

### verifier/verifier.go

- 解析 VLESS 分享链接（UUID、地址、端口、ws、tls、host、path 等）
- 生成 xray JSON 配置（vless + ws + tls 出站，http 入站）
- 启动临时 xray 实例，通过代理访问 verifyURL 验证 IP 可用性
- 依赖外部二进制 `./xray`（缺失即退出，与 `./nexttrace` 一致）


### cloudflare/cloudflare.go

- 调用 Cloudflare API 更新 DNS 记录
- 使用 PATCH 方法更新 A 记录
- 设置 TTL 为 1（自动）

### providers/

- 定义 Provider 接口
- 实现多个数据源：
  - uouin.go：uouin.com API
  - ipdb.go：IPDB 数据源
  - zhixuanwang.go：智选网数据源

---

## 十、使用方法

### 依赖准备

启动前需要准备以下二进制文件（放在项目根目录）：

| 文件 | 用途 | 下载地址 |
|------|------|----------|
| `./nexttrace` | 路由追踪（必需） | https://github.com/nxtrace/NTrace-core/releases |
| `./xray` | VLESS 验证（仅当配置了 `vless` 时必需） | https://github.com/XTLS/Xray-core/releases |

下载后需重命名并赋予执行权限：

```bash
mv nexttrace_linux_amd64 nexttrace && chmod +x nexttrace
unzip Xray-linux-64.zip xray -d ./ && chmod +x xray
```

> Docker 镜像会自动下载这两个二进制，无需手动准备。

### 启动精简模式

```bash
./cf-optimizer
```

### 启动完整模式

```bash
./cf-optimizer -full
```

### 访问 hosts 文件

```
http://localhost:37377/gethosts
```
### 构建
```
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o cf-optimizer-linux-arm64 .
```
---

## 十一、技术特点

1. **并发处理**：使用 goroutine 和 sync.WaitGroup 实现并发 IP 获取和测试
2. **数据持久化**：使用 SQLite 存储 IP 历史数据
3. **路由分析**：集成 nexttrace 进行精确的地理位置分析
4. **VLESS 可用性验证**：通过临时 xray 实例验证 IP 是否真正可用于代理，解决 ping 通但无法代理的问题，失败自动回退
5. **自动化**：定时任务自动执行 IP 获取、测试、验证和 DNS 更新
6. **容错机制**：对网络请求和命令执行进行错误处理，所有 IP 验证失败时保留原 DNS 记录

---

## 十二、适用场景

- 需要优化 Cloudflare CDN 访问速度的场景
- 需要根据地理位置选择最优 IP 的应用
- 需要自动维护 DNS 记录的服务
- 对网络延迟敏感的应用
- 需要确保优选 IP 真正可用于 VLESS 代理的场景

---

## 十三、依赖项

### 外部二进制

- `./nexttrace`：路由追踪工具（[nxtrace/NTrace-core](https://github.com/nxtrace/NTrace-core)）
- `./xray`：代理工具，用于 VLESS 验证（[XTLS/Xray-core](https://github.com/XTLS/Xray-core)，仅当配置了 `vless` 时需要）

### Go 模块

- `github.com/glebarez/go-sqlite`：SQLite 数据库驱动
- `gopkg.in/yaml.v3`：YAML 配置文件解析

---

## 总结

这个规则系统的核心思想是通过路由追踪、地理位置匹配和 VLESS 可用性验证，为不同域名选择经过特定地理位置的最优 Cloudflare IP，从而优化网络访问性能。系统支持灵活的规则配置、多数据源集成、自动延迟测试、VLESS 验证回退和 DNS 更新，是一个完整的自动化 IP 优选解决方案。
