# RuneShell / 符令迷城

> A terminal deckbuilding roguelike where cards, equipment, relics, and bad ideas all fit in one window.

一个认真做规则、认真做构筑、认真把多人合作也塞进终端 UI 的卡牌肉鸽。

`Go` `Bubble Tea` `Deckbuilder` `Roguelike` `LAN Co-op` `Terminal UI`

## Terminal Showcase

```text
+--------------------------------------------------------------------------------+
| RuneShell / 符令迷城                                                           |
| Act 2 - Elite                                                                  |
+-----------------------------------+--------------------------------------------+
| Hand                              | Enemy Intent                               |
| > Shield Slam      cost 1         | Brass Warden     ATK 18 + Burn            |
|   Relay Bulwark    cost 1         | Ash Weaver       Block 12                 |
|   Ember Draft      cost 2         |                                            |
|   Tactical Sip     potion         | Party                                       |
|                                   | Vanguard      HP 41/57  Block 16  Energy 2/3 |
| Target: Brass Warden              | Arcanist      HP 28/39  Block  4  Energy 3/3 |
|                                   |                                            |
| Relics                            | Combat Log                                 |
| - Battlefield Manual              | - Guard +12                                 |
| - Relay Rations                   | - Burn applied to front enemy               |
| - Brass Compass                   | - Host suggests focus fire on seat 1 target |
+-----------------------------------+--------------------------------------------+
| Enter: play card   Z: potion mode   Tab: operations / inspect / chat          |
+--------------------------------------------------------------------------------+
```

这不是精确截图，而是项目想传达的气质样板：

- 一眼能看到战局、资源、目标和日志
- 终端布局是游戏界面，不是调试输出
- 多人合作的信息也应该自然地出现在同一个视图里

## One-Line Pitch

如果《杀戮尖塔》、终端界面、局域网联机、装备联动、还有“我这回合应该真能赢了吧”这种情绪放在同一个锅里煮，出来的大概就是 RuneShell。

## Name

- 英文名：RuneShell
- 中文名：符令迷城

这组名字对应的就是项目气质本身：

- Rune：符文、卡牌、状态、遗物、魔法效果
- Shell：终端、命令、输入、操作感
- 符令迷城：一边打牌，一边发号施令，一边往肉鸽地图更深处走

## Feature Snapshot

| Feature | Status | Notes |
| --- | --- | --- |
| 单机主流程 | Ready | 主线 3 幕，可完整跑通一局 |
| 无尽模式 | Ready | 继续往后打，测试构筑上限 |
| 职业系统 | Ready | 当前已有铁卫、秘术师 |
| 装备系统 | Ready | weapon / armor / accessory 三槽位构筑联动 |
| 遗物 / 药水 / 事件 | Ready | 已接入奖励、商店、事件决策链 |
| 图鉴 / 局外成长 | Ready | 可浏览内容并推进解锁 |
| 局域网多人合作 | Ready | 直接内嵌在 Bubble Tea 主界面 |
| 内容扩展 | Ongoing | JSON 驱动，持续加牌、加敌人、加事件 |

## At A Glance

- 终端里的卡牌肉鸽，不是日志浏览器
- 构筑里有卡、装备、遗物、药水四层联动
- 单机可玩，多人可联，而且都在同一个 TUI 里
- 规则内核和内容系统分层清楚，方便持续扩展
- 最近重点正在把多人操作手感继续往单机体验靠拢

## Why This Project Is Fun

它的乐趣不只是“终端里也能打牌”，而是几种味道叠在一起：

### 1. 卡牌战斗是主菜

- 抽牌
- 规划能量
- 计算目标
- 处理状态
- 想清楚这张牌现在打还是留到下回合

你不是在点一个自动战斗按钮，而是在用一小把资源解决一个不断变坏的问题。

### 2. 装备不是贴纸，是构筑的一部分

这里的装备不是“攻击 +3，生命 +5”就收工了。

- `weapon` 影响输出路径
- `armor` 影响生存与防线
- `accessory` 影响资源、循环、状态联动

你拿到一件装备，常常不是“数值更高了”，而是“我整个牌组的思路都得变一下”。

### 3. 终端味不是噱头

这个项目不是把图形界面压扁之后硬塞进终端。

- 页面会按窗口宽度调整布局
- 战斗、图鉴、成长、商店都做了视口适配
- 键盘操作是默认路径，不是备选方案

它想做的是一款真的适合在终端里玩的游戏，而不是一张终端皮肤。

### 4. 多人不是外挂模式

局域网联机已经接进主 UI 里了。

- 主菜单直接进入多人模式
- 创建 / 加入房间是中文表单引导
- 房间里支持聊天、建议动作、房主决策、协作战斗
- 最近还把多人战斗和非战斗阶段都继续往单机操作手感上拉齐

这意味着你不是在操作一个聊天机器人协议界面，而是在玩一款真的有联机房间的 TUI 游戏。

## Game Loop

1. 选择职业开局
2. 在地图节点之间做路线决策
3. 进入战斗，用手牌、能量、药水和状态解决眼前问题
4. 从奖励、商店、事件、营火里不断修正构筑
5. 打完 3 幕主线，或者转进无尽模式继续挑战

当前可玩职业：

- 铁卫
- 秘术师

当前核心模式：

- 主线模式
- 无尽模式

## Why It Feels Different

- 内容不是硬编码堆起来的：职业、卡牌、遗物、药水、装备、事件都走 JSON
- 规则内核优先：战斗、地图、奖励、成长都尽量做成可测试、可复现、可扩展
- 多人合作是原生功能：不是外接脚本，不是额外开个古早控制台
- 终端视图不是摆设：窄窗口下也尽量保证战斗和列表可读性

## Quick Start

### 直接开始

```bash
go run ./cmd/cmdcards
```

或者：

```bash
go run ./cmd/cmdcards play
```

### 校验内容

```bash
go run ./cmd/cmdcards validate-content
```

### 跑 smoke

```bash
go run ./cmd/cmdcards smoke --mode story --class vanguard --seed 1
go run ./cmd/cmdcards smoke --mode endless --class vanguard --seed 1
```

## Multiplayer, But Still A Game

如果你想在局域网里找朋友一起顶住精英怪，这项目已经支持两种方式。

### 方式一：游戏内进入多人模式

直接从主菜单进入 `多人模式`，创建房间或加入房间。

当前体验包括：

- 中文创建 / 加入表单
- 房间直接渲染在 Bubble Tea 界面里
- 聊天、建议动作、房主决策
- 战斗和非战斗阶段都在往单机式方向键主操作靠拢

### 方式二：命令行 host / join

```bash
go run ./cmd/cmdcards host --port 7777 --name Host --class vanguard
go run ./cmd/cmdcards join --addr 127.0.0.1:7777 --name Guest --class arcanist
```

## Controls In Plain Language

- `上下`：选项目
- `左右`：翻页、切目标、切面板
- `回车`：确认
- `Tab`：切焦点
- `Esc`：返回
- `e`：结束回合
- `z`：战斗里切到药水模式

多人模式最近这部分又往前推了一步：

- 战斗阶段可以直接选手牌 / 药水 / 目标
- 非战斗阶段可以直接在地图、奖励、商店、事件、营火等页面里用方向键选项
- 战斗里 `Tab` 会在 `操作区 / 检视区 / 聊天框` 之间循环

一句话总结：你操作的是游戏，不是协议。

## Project Structure

```text
cmd/cmdcards        # CLI 入口
internal/app        # 页面状态机与输入分发
internal/ui         # 纯渲染层
internal/engine     # 战斗 / 地图 / 奖励 / 规则内核
internal/content    # JSON 内容加载与校验
internal/netplay    # 局域网联机
internal/storage    # 存档
```

## Tech Stack

- Go 1.26
- Bubble Tea
- Bubbles
- Lip Gloss

## Development Progress

当前状态不是“玩法只剩概念图”，而是已经有稳定骨架、可玩流程和持续扩展能力。

已完成：

- 可运行 v1 骨架
- 主线 3 幕
- 无尽模式
- 战斗状态系统
- 装备比较 / 替换 / 估值链路
- 奖励 / 商店 / 事件 / 营火等决策流程
- 图鉴与局外成长
- 本地存档
- 局域网多人合作基础体验
- 响应式终端布局

正在继续推进：

- 新职业
- 更深的卡组流派
- 更多装备与事件联动
- 更完整的多人合作机制
- 更多内容量和更成熟的数值平衡

## Roadmap

### Near Term

- 扩充职业卡池
- 增加更多精英 / Boss 机制
- 继续打磨多人房间交互
- 补更多 UI 回归测试和内容验证

### Mid Term

- 新职业
- 更多事件模板和装备池
- 更明显的构筑分支
- 更完整的无尽模式增长规则

### Long Term

- 更强的职业差异化
- 更丰富的合作玩法
- 更完整的内容量与节奏打磨

## Who This Is For

如果你符合下面任意一条，这项目大概率对你有效：

- 你喜欢卡牌肉鸽
- 你愿意在终端里玩真的游戏，而不是只能看日志
- 你对 Go 写游戏规则内核这件事感兴趣
- 你喜欢“系统先搭对，再慢慢往里塞内容”的项目

## A Very Accurate Scene

> 你盯着终端，手里 3 点能量，前排精英快开大，背包里还剩一瓶药。你看着自己那套精心构筑、理论上已经开始起飞的牌组，认真地想：这回合应该稳了。五秒之后，你开始研究自己为什么会在商店里买下一件会发光但显然不该买的饰品。

欢迎来到 RuneShell。

欢迎进入符令迷城。