# cmd-cards 实现冻结基线

> 本文档用于当前实现轮次的工程冻结，优先级低于长期设计主文档 `docs/DESIGN_PLAN.md`。

## 1. 架构职责边界

- `cmd/cmdcards` 只负责 CLI 入口和命令分发。
- `internal/app` 只负责页面状态机、输入分发、流程跳转和存档/恢复触发。
- `internal/ui` 只负责渲染，不承载规则判断。
- `internal/engine` 只负责规则内核、战斗、地图、奖励、事件、商店、结算与 smoke。
- `internal/content` 只负责 JSON 内容定义、加载、校验和索引，不写运行时行为。
- `internal/netplay` 只负责联机协议、seat-specific 状态、房间快照、持久化和重连。
- `internal/storage` 只负责 profile/run 存档与兼容读取。

## 2. 不可破坏约束

- 继续使用 typed opcode，不在 JSON 中嵌脚本。
- 继续坚持 data-driven 内容扩展，优先通过 JSON 和少量装配代码加内容。
- `engine` 不依赖 TUI 组件，`ui` 不写规则，`app` 不硬编码内容数值。
- 凡是“给牌加额外效果”的需求，优先复用 `augment_card`，不要为单个效果再开专用分支。
- 装备继续保持三槽位：`weapon`、`armor`、`accessory`。
- 联机继续使用共享战场 + seat 私有状态，不回退成单状态伪多人。
- 交互继续键盘优先，窄窗口可读性保持一等优先级。

## 3. 第一轮内容包

### 3.1 `augment_card` build 方向

- 抽牌型：使用时抽牌、命中后抽牌、击杀后抽牌。
- 能量型：使用时返能、条件返能、回合内资源回收。
- 重复型：下一张牌重复、指定 tag 重复、对关键牌放大收益。
- 状态型：附带 `burn`、`weak`、`vulnerable`、`shielded` 等效果。
- 保留型：本回合未打出则下回合降费或增强。

### 3.2 事件 / 工坊 / 特殊商店模板

- 事件模板优先复用现有 `augment_card` 选牌链路。
- 工坊模板优先作为付费构筑服务，不新增独立 UI/存储分支。
- 特殊商店模板优先复用 `DeckActionPlan`，让“买服务 -> 选牌 -> 附加效果”成为统一链路。
- 第一轮内容只要求形成 3 到 5 条可识别 build 方向，不追求职业扩张或大规模数值平衡。

## 4. 第二轮联机协作内容包

- 协防：给队友格挡或减伤。
- 分伤：本回合替队友承担部分伤害。
- 集火：若队友已命中过目标，则追加收益。
- 共振：队伍共享小额抽牌、能量或资源转化。
- 联机事件：以双人投票或分歧决策为主，避免复杂新协议。

## 5. 本轮验收命令

```bash
go test ./...
go run ./cmd/cmdcards validate-content
go run ./cmd/cmdcards smoke --mode story --class vanguard --seed 1
go run ./cmd/cmdcards smoke --mode endless --class vanguard --seed 1
```

## 6. 本轮验收重点

- 事件 -> 选牌 -> `augment_card` 生效 -> 存档恢复。
- 商店 service -> `DeckActionPlan` -> 扣费 -> 附魔 -> offer 消耗。
- 联机 reward/event/shop/deck_action 在 seat 私有状态下的推进、广播、重连恢复。
- `roomSnapshot` / UI 对 augment、未读聊天、host 权限、seat 状态的显示一致性。
- host transfer、saved room restore、chat log、checkpoint/persist 的组合回归。

