# Cloudflare IP 优选域名管理系统 - 规则系统文档

## 项目概述

cf-optimizer 是一个自动化的 Cloudflare IP 优选系统，通过路由追踪、延迟测试和 DNS 自动更新，为不同域名选择经过特定地理位置的最优 Cloudflare IP。

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
从多个数据源获取 IP
    ↓
去重处理
    ↓
过滤数据库中已存在的 IP
    ↓
只处理新 IP
```

### 2. IP 分组阶段

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

### 3. IP 优选阶段

#### 精简模式

```
从数据库获取每个分组的前 5 个 IP
    ↓
对每个 IP 进行 10 次延迟测试
    ↓
计算平均延迟
    ↓
选择延迟最低的 IP
    ↓
更新 DNS
```

#### 完整模式

```
从数据库获取每个分组的前 5 个 IP
    ↓
对每个 IP 进行 5 次延迟测试
    ↓
计算平均延迟
    ↓
选择延迟最低的 IP
    ↓
更新 DNS
```

### 4. DNS 更新阶段

```
根据 hostMap 确定域名-分组关系
    ↓
获取每个分组的最优 IP
    ↓
调用 Cloudflare API
    ↓
更新 DNS A 记录
```

---

## 五、实际应用示例

### 场景：用户访问 sg.yx.meiyoukaoshang.dpdns.org

1. **系统查找**：该域名使用 `SG_GD` 分组
2. **IP 选择**：从数据库获取 SG_GD 分组的最优 IP
3. **路由验证**：该 IP 的路由路径包含 "新加坡" 和 "广东"
4. **DNS 解析**：用户被解析到该 IP
5. **访问路径**：用户 → 广东 → 新加坡 → Cloudflare

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

### 注意事项

- DNS 记录 ID 需要从 Cloudflare 控制台获取
- 地理位置名称需与 nexttrace 返回的名称一致
- 建议先测试路由路径再配置规则

---

## 八、运行模式对比

### 精简模式

- **数据源**：仅使用 uouin.com
- **延迟测试**：每个 IP 测试 10 次
- **更新频率**：每 2 小时
- **资源消耗**：中等
- **适用场景**：快速部署、低资源环境

### 完整模式

- **数据源**：uouin.com、ipdb、智选网
- **延迟测试**：每个 IP 测试 5 次
- **更新频率**：每 1 小时
- **资源消耗**：中等
- **适用场景**：需要最优性能

---

## 九、核心模块说明

### database/database.go

- 使用 SQLite 存储 IP 数据
- 提供插入、查询、过滤等功能
- 支持按组获取最新 IP

### tracer/tracer.go

- 使用 nexttrace 工具进行路由追踪
- 解析路由路径中的地理位置信息
- 根据配置规则对 IP 进行分组

### latency/latency.go

- 使用 curl 命令测试连接延迟
- 通过 --resolve 参数指定 IP 地址
- 解析 time_connect 值获取延迟

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

---

## 十一、技术特点

1. **并发处理**：使用 goroutine 和 sync.WaitGroup 实现并发 IP 获取和测试
2. **数据持久化**：使用 SQLite 存储 IP 历史数据
3. **路由分析**：集成 nexttrace 进行精确的地理位置分析
4. **自动化**：定时任务自动执行 IP 获取、测试和 DNS 更新
5. **容错机制**：对网络请求和命令执行进行错误处理

---

## 十二、适用场景

- 需要优化 Cloudflare CDN 访问速度的场景
- 需要根据地理位置选择最优 IP 的应用
- 需要自动维护 DNS 记录的服务
- 对网络延迟敏感的应用

---

## 十三、依赖项

- `github.com/mattn/go-sqlite3`：SQLite 数据库驱动
- `gopkg.in/yaml.v3`：YAML 配置文件解析

---

## 总结

这个规则系统的核心思想是通过路由追踪和地理位置匹配，为不同域名选择经过特定地理位置的最优 Cloudflare IP，从而优化网络访问性能。系统支持灵活的规则配置、多数据源集成、自动延迟测试和 DNS 更新，是一个完整的自动化 IP 优选解决方案。
