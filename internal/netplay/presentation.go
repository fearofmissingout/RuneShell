package netplay

import (
	"fmt"
	"strings"

	"cmdcards/internal/engine"
)

type phasePresentation struct {
	PhaseHint    string
	ControlLabel string
	RoleNote     string
	Commands     []string
	Examples     []string
}

type personalPhasePresentationConfig struct {
	Pending         bool
	ActiveControl   string
	WaitingControl  string
	ActiveRole      string
	WaitingRole     string
	ActiveHint      string
	WaitingHint     string
	ActiveCommands  []string
	WaitingCommands []string
	ActiveExamples  []string
	WaitingExamples []string
}

type flowPhasePresentationConfig struct {
	Owns            bool
	OwnerControl    string
	WaitingControl  string
	OwnerRole       string
	WaitingRole     string
	OwnerHint       string
	WaitingHint     string
	OwnerCommands   []string
	WaitingCommands []string
	OwnerExamples   []string
	WaitingExamples []string
}

func personalPhaseText(active bool, activeText string, waitingText string) string {
	if active {
		return activeText
	}
	return waitingText
}

func flowOwnerPhaseText(active bool, ownerText string, waitingText string) string {
	if active {
		return ownerText
	}
	return waitingText
}

func applyPersonalPhasePresentation(presentation *phasePresentation, config personalPhasePresentationConfig, appendHint func(string)) {
	presentation.ControlLabel = personalPhaseText(config.Pending, config.ActiveControl, config.WaitingControl)
	if presentation.RoleNote == "" {
		presentation.RoleNote = personalPhaseText(config.Pending, config.ActiveRole, config.WaitingRole)
	}
	presentation.Commands = append([]string{}, config.WaitingCommands...)
	presentation.Examples = append([]string{}, config.WaitingExamples...)
	if config.Pending {
		presentation.Commands = append([]string{}, config.ActiveCommands...)
		presentation.Examples = append([]string{}, config.ActiveExamples...)
	}
	appendHint(personalPhaseText(config.Pending, config.ActiveHint, config.WaitingHint))
}

func applyFlowPhasePresentation(presentation *phasePresentation, config flowPhasePresentationConfig, appendHint func(string)) {
	presentation.ControlLabel = flowOwnerPhaseText(config.Owns, config.OwnerControl, config.WaitingControl)
	if presentation.RoleNote == "" {
		presentation.RoleNote = flowOwnerPhaseText(config.Owns, config.OwnerRole, config.WaitingRole)
	}
	presentation.Commands = append([]string{}, config.WaitingCommands...)
	presentation.Examples = append([]string{}, config.WaitingExamples...)
	if config.Owns {
		presentation.Commands = append([]string{}, config.OwnerCommands...)
		presentation.Examples = append([]string{}, config.OwnerExamples...)
	}
	appendHint(flowOwnerPhaseText(config.Owns, config.OwnerHint, config.WaitingHint))
}

func (s *server) buildLobbyPhasePresentationLocked(player *roomPlayer, isHost bool, offline []string, waitingOn []string, presentation *phasePresentation, appendHint func(string)) {
	if isHost {
		presentation.ControlLabel = "房主房间设置"
		if presentation.RoleNote == "" {
			presentation.RoleNote = "你负责房间设置，可以开始新的一局，也可以清理离线保留席位。"
		}
		presentation.Commands = []string{"class <id>", "ready", "mode <story|endless>", "seed <n>", "start", "drop <seat|all>", "chat <text>", "abandon", "quit"}
	} else {
		presentation.ControlLabel = "席位准备"
		if presentation.RoleNote == "" {
			presentation.RoleNote = "选择职业、切换准备状态，然后等待房主开始。"
		}
		presentation.Commands = []string{"class <id>", "ready", "chat <text>", "quit"}
	}
	classID := ""
	if player != nil {
		classID = player.ClassID
	}
	if classID == "" {
		classes := classIDs(s.lib)
		if len(classes) > 0 {
			classID = classes[0]
		}
	}
	if classID != "" {
		presentation.Examples = append(presentation.Examples, fmt.Sprintf("class %s", classID))
	}
	presentation.Examples = append(presentation.Examples, "ready")
	if isHost {
		presentation.Examples = append(presentation.Examples, fmt.Sprintf("seed %d", s.seed), fmt.Sprintf("mode %s", s.mode))
		for seat, id := range s.order {
			offlinePlayer := s.players[id]
			if offlinePlayer == nil || offlinePlayer.Connected {
				continue
			}
			presentation.Examples = append(presentation.Examples, fmt.Sprintf("drop %d", seat+1))
			break
		}
		if len(offline) > 1 {
			presentation.Examples = append(presentation.Examples, "drop all")
		}
		if s.canStartRunLocked() {
			presentation.Examples = append(presentation.Examples, "start")
		}
	}
	presentation.Examples = append(presentation.Examples, "chat 准备好了就开")
	if len(waitingOn) > 0 {
		appendHint("正在等待已连接玩家准备。")
	}
	if len(offline) > 0 {
		if isHost {
			appendHint("离线席位会阻止开始新的一局。让对方用相同名字重连，或在大厅使用 `drop <seat>` 清理。")
		} else {
			appendHint("离线席位需要先重连，房主才能开始新的一局。")
		}
	} else if isHost {
		appendHint("当前在线玩家都已准备，可以随时开始。")
	} else {
		appendHint("请选择职业并切换为准备状态。")
	}
}

func (s *server) buildMapPhasePresentationLocked(selfID string, isHost bool, waitingOn []string, presentation *phasePresentation, appendHint func(string)) {
	presentation.ControlLabel = "共享路线投票"
	if presentation.RoleNote == "" {
		presentation.RoleNote = "每个已连接席位都要为下一个节点投票。当前所有席位选好路线后，房间才会推进。"
	}
	presentation.Commands = []string{"node <index>", "chat <text>", "quit"}
	if s.run == nil {
		presentation.Examples = []string{"chat 等路线中"}
	} else if reachable := engine.ReachableNodes(s.run); len(reachable) == 0 {
		presentation.Examples = []string{"chat 走哪条路？"}
	} else {
		presentation.Examples = []string{"node 1", "chat 我投 1 号点"}
	}
	if isHost {
		appendHint("请提交你的路线投票。所有已连接席位都投完后，房间会自动推进。")
	} else {
		appendHint("请提交你的路线投票，然后等待其余已连接席位。")
	}
	if state := s.seatStateLocked(selfID); state != nil && state.MapVote > 0 {
		appendHint("你的投票已提交：" + s.mapVoteChoiceLabelLocked(state.MapVote) + "。")
		if len(waitingOn) > 0 {
			appendHint(fmt.Sprintf("还在等待 %d 个已连接席位投票。", len(waitingOn)))
		}
		return
	}
	if len(waitingOn) > 1 {
		appendHint(fmt.Sprintf("还有 %d 个已连接席位尚未投票。", len(waitingOn)))
	}
}

func (s *server) buildCombatPhasePresentationLocked(selfID string, offline []string, waitingOn []string, presentation *phasePresentation, appendHint func(string)) {
	presentation.ControlLabel = "个人战斗操作"
	if presentation.RoleNote == "" {
		presentation.RoleNote = "战斗阶段采用席位私有的手牌、牌堆、药水和能量；战场、敌人和结束回合投票仍然由全房间共享。"
	}
	presentation.Commands = []string{"play <card#> [enemy|ally <target#>]", "potion <slot#> [enemy|ally <target#>]", "end", "chat <text>", "quit"}
	if s.run == nil || s.combat == nil {
		presentation.Examples = []string{"chat 准备结束回合", "end"}
	} else {
		view := buildCombatSnapshot(s.lib, s.run, s.combat, selfID, s.order, s.players)
		if view != nil && len(view.Hand) > 0 {
			presentation.Examples = append(presentation.Examples, combatCommandExample(view.Hand[0]))
		}
		if view != nil && len(view.Potions) > 0 {
			presentation.Examples = append(presentation.Examples, "potion 1")
		}
		presentation.Examples = append(presentation.Examples, "chat 集火左边敌人", "end")
	}
	if len(waitingOn) > 0 {
		appendHint("先出牌，再结束回合。所有已连接席位都投票后，敌方回合才会开始。")
	} else {
		appendHint("所有已连接席位都已准备结束回合。")
	}
	if len(offline) > 0 {
		appendHint("离线席位会保留位置，并在结束回合投票中按自动准备处理。")
	}
}

func (s *server) buildRewardPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	pending := s.seatRewardPendingLocked(selfID)
	config := personalPhasePresentationConfig{
		Pending:         pending,
		ActiveControl:   "个人奖励选择",
		WaitingControl:  "等待其他人完成奖励",
		ActiveRole:      "你需要处理自己的战后奖励。所有已连接席位都完成后，房间才会继续推进。",
		WaitingRole:     "你的奖励已经处理完，正在等待其他已连接席位完成。",
		ActiveHint:      "请完成你的个人奖励选择。",
		WaitingHint:     "正在等待其余席位处理奖励。",
		ActiveCommands:  []string{"take <card#>", "skip", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		WaitingExamples: []string{"chat 我拿第一张"},
	}
	if pending {
		if reward := s.seatRewardStateLocked(selfID); reward != nil && len(reward.CardChoices) > 0 {
			config.ActiveExamples = []string{"take 1", "skip", "chat 我拿 1 号卡"}
		} else {
			config.ActiveExamples = []string{"skip", "chat 我跳过奖励"}
		}
	}
	applyPersonalPhasePresentation(presentation, config, appendHint)
}

func (s *server) buildEventPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	pending := s.seatEventPendingLocked(selfID)
	config := personalPhasePresentationConfig{
		Pending:         pending,
		ActiveControl:   "个人事件选择",
		WaitingControl:  "等待其他人完成事件",
		ActiveRole:      "你要决定自己的事件结果。所有已连接席位都处理完事件后，共享地图才会继续。",
		WaitingRole:     "你的事件已经处理完，正在等待其他已连接席位完成。",
		ActiveHint:      "请完成你的个人事件选择。",
		WaitingHint:     "正在等待其余席位处理事件。",
		ActiveCommands:  []string{"choose <index>", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		ActiveExamples:  []string{"choose 1", "chat 我选 1"},
		WaitingExamples: []string{"chat 你也可以选 1"},
	}
	if pending {
		if eventState := s.seatEventStateLocked(selfID); eventState == nil || len(eventState.Event.Choices) == 0 {
			config.ActiveExamples = []string{"chat 你觉得选哪个？"}
		}
	}
	applyPersonalPhasePresentation(presentation, config, appendHint)
}

func (s *server) buildShopPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	pending := s.seatShopPendingLocked(selfID)
	config := personalPhasePresentationConfig{
		Pending:         pending,
		ActiveControl:   "个人商店",
		WaitingControl:  "等待其他人离开商店",
		ActiveRole:      "你拥有自己的商店库存和金币状态。完成后离开，地图才会继续推进。",
		WaitingRole:     "你的商店阶段已完成，正在等待其他已连接席位离开。",
		ActiveHint:      "从你的个人商店购买，或完成后离开。",
		WaitingHint:     "正在等待其余席位离开商店。",
		ActiveCommands:  []string{"buy <index>", "leave", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		WaitingExamples: []string{"chat 商店里有你想要的吗？"},
	}
	if pending {
		if shop := s.seatShopStateLocked(selfID); shop != nil && len(shop.Offers) > 0 {
			config.ActiveExamples = []string{"buy 1", "leave", "chat 我买 1 号"}
		} else {
			config.ActiveExamples = []string{"leave", "chat 我先离开商店"}
		}
	}
	applyPersonalPhasePresentation(presentation, config, appendHint)
}

func (s *server) buildRestPhasePresentationLocked(isHost bool, presentation *phasePresentation, appendHint func(string)) {
	presentation.ControlLabel = phaseHostWaitHint(isHost, "共享篝火选择")
	if !isHost {
		presentation.ControlLabel = "等待篝火选择"
	}
	if presentation.RoleNote == "" {
		presentation.RoleNote = phaseHostWaitHint(isHost, "由你为全房间决定篝火行动。")
		if !isHost {
			presentation.RoleNote = "你正在等待房主处理篝火选择。"
		}
	}
	if isHost {
		presentation.Commands = []string{"heal", "upgrade", "chat <text>", "quit"}
		presentation.Examples = []string{"heal", "upgrade", "chat 篝火升级"}
	} else {
		presentation.Commands = []string{"chat <text>", "quit"}
		presentation.Examples = []string{"chat 回血还是升级？"}
	}
	appendHint(phaseHostWaitHint(isHost, "由房主选择回血或升级。"))
}

func (s *server) buildEquipmentPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	applyFlowPhasePresentation(presentation, flowPhasePresentationConfig{
		Owns:            s.seatOwnsFlowLocked(selfID),
		OwnerControl:    "装备替换",
		WaitingControl:  "等待装备选择",
		OwnerRole:       "由你决定是否用这件新装备替换当前槽位的装备。",
		WaitingRole:     "另一个席位正在处理装备替换提示。",
		OwnerHint:       "请确认是否装备这件新物品。",
		WaitingHint:     "正在等待其他席位处理装备选择。",
		OwnerCommands:   []string{"take", "skip", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		OwnerExamples:   []string{"take", "skip", "chat 我准备装上"},
		WaitingExamples: []string{"chat 这件要拿吗？"},
	}, appendHint)
}

func (s *server) buildDeckActionPhasePresentationLocked(selfID string, presentation *phasePresentation, appendHint func(string)) {
	ownerExamples := []string{"back", "chat backing out"}
	if len(s.deckActionIndexes) > 0 {
		ownerExamples = []string{"choose 1", "back", "chat picking card 1"}
	}
	applyFlowPhasePresentation(presentation, flowPhasePresentationConfig{
		Owns:            s.seatOwnsFlowLocked(selfID),
		OwnerControl:    "牌组操作",
		WaitingControl:  "等待牌组操作",
		OwnerRole:       "由你选择当前牌组操作要影响的卡牌。",
		WaitingRole:     "另一个席位正在处理牌组操作。",
		OwnerHint:       "请选择当前牌组操作要作用的卡牌。",
		WaitingHint:     "正在等待其他席位处理牌组操作。",
		OwnerCommands:   []string{"choose <index>", "back", "chat <text>", "quit"},
		WaitingCommands: []string{"chat <text>", "quit"},
		OwnerExamples:   ownerExamples,
		WaitingExamples: []string{"chat 选 1 号怎么样？"},
	}, appendHint)
}

func (s *server) buildSummaryPhasePresentationLocked(isHost bool, presentation *phasePresentation, appendHint func(string)) {
	if isHost {
		presentation.ControlLabel = "房主结算决定"
		if presentation.RoleNote == "" {
			presentation.RoleNote = "你可以在这里开始新的一局，或直接放弃当前房间存档。"
		}
		presentation.Commands = []string{"new", "abandon", "chat <text>", "quit"}
		presentation.Examples = []string{"new", "abandon", "chat 要重开吗？"}
	} else {
		presentation.ControlLabel = "等待房主决定"
		if presentation.RoleNote == "" {
			presentation.RoleNote = "你正在等待房主决定继续下一局还是关闭房间。"
		}
		presentation.Commands = []string{"chat <text>", "quit"}
		presentation.Examples = []string{"chat gg"}
	}
	appendHint(phaseHostWaitHint(isHost, "房主可以开始新的一局或放弃当前房间。"))
}

func (s *server) phasePresentationLocked(selfID string, waitingOn []string) phasePresentation {
	isHost := selfID == s.hostID
	offline := s.offlineSeatSummariesLocked()
	presentation := phasePresentation{}

	if note := s.transferNoteLocked(selfID); note != "" {
		presentation.PhaseHint = note
	}
	if s.hostTransfer != nil {
		switch selfID {
		case s.hostTransfer.FromID:
			presentation.RoleNote = "房主转移正在等待处理中。等待目标席位接受，或使用 cancel-host 取消。"
			presentation.Examples = []string{"chat 房主转移处理中", "cancel-host"}
		case s.hostTransfer.ToID:
			presentation.RoleNote = "你收到一个房主转移请求。接受后会获得房间控制权，拒绝则保持当前房主不变。"
			presentation.Examples = []string{"accept-host", "deny-host", "chat 我来接房主"}
		default:
			presentation.Examples = []string{"chat 等房主转移结果"}
		}
	}

	appendHint := func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		if presentation.PhaseHint == "" {
			presentation.PhaseHint = text
			return
		}
		presentation.PhaseHint += " " + text
	}

	if s.restoredFromSave && len(offline) > 0 {
		if isHost {
			appendHint("已从磁盘恢复房间。等待离线席位用相同名字重连，或保留这些离线席位继续游戏。")
		} else {
			appendHint("该房间已从磁盘恢复。离线席位可以使用相同名字重连并取回自己的位置。")
		}
	}

	player := s.players[selfID]
	switch s.phase {
	case phaseLobby:
		s.buildLobbyPhasePresentationLocked(player, isHost, offline, waitingOn, &presentation, appendHint)
	case phaseMap:
		s.buildMapPhasePresentationLocked(selfID, isHost, waitingOn, &presentation, appendHint)
		presentation.ControlLabel = personalPhaseText(s.seatMapVoteSubmittedLocked(selfID), "路线投票已提交", "共享路线投票")
		if s.seatMapVoteSubmittedLocked(selfID) {
			if state := s.seatStateLocked(selfID); state != nil && state.MapVote > 0 {
				appendHint("你的路线投票已提交：" + s.mapVoteChoiceLabelLocked(state.MapVote) + "。")
			}
			appendHint("正在等待其余已连接席位。")
		}
	case phaseCombat:
		s.buildCombatPhasePresentationLocked(selfID, offline, waitingOn, &presentation, appendHint)
	case phaseReward:
		s.buildRewardPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseEvent:
		s.buildEventPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseShop:
		s.buildShopPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseRest:
		s.buildRestPhasePresentationLocked(isHost, &presentation, appendHint)
	case phaseEquipment:
		s.buildEquipmentPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseDeckAction:
		s.buildDeckActionPhasePresentationLocked(selfID, &presentation, appendHint)
	case phaseSummary:
		s.buildSummaryPhasePresentationLocked(isHost, &presentation, appendHint)
	default:
		presentation.Commands = []string{"chat <text>", "quit"}
	}

	presentation.Commands = s.appendTransferCommandsLocked(selfID, presentation.Commands)
	if s.hostTransfer == nil {
		presentation.Examples = compactStrings(s.appendTransferExamplesLocked(selfID, presentation.Examples))
	}
	return presentation
}

func (s *server) phaseResumeLinesLocked(selfID string) []string {
	switch s.phase {
	case phaseLobby:
		return []string{fmt.Sprintf("模式 %s，种子 %d。", s.mode, s.seed)}
	case phaseMap:
		if s.run != nil {
			return []string{fmt.Sprintf("第 %d 幕，下一层 %d，金币 %d。", s.run.Act, s.run.CurrentFloor+1, s.seatGoldLocked(selfID))}
		}
	case phaseCombat:
		if s.combat != nil {
			energy := s.combat.Player.Energy
			maxEnergy := s.combat.Player.MaxEnergy
			if seatIndex := s.playerSeatIndexLocked(selfID); seatIndex >= 0 {
				if actor := engine.ActorForSeat(s.combat, seatIndex); actor != nil {
					energy = actor.Energy
					maxEnergy = actor.MaxEnergy
				}
			}
			return []string{fmt.Sprintf("战斗回合 %d，能量 %d/%d。", s.combat.Turn, energy, maxEnergy)}
		}
	case phaseReward:
		return []string{"奖励选择仍在等待处理中。"}
	case phaseEvent:
		return []string{"事件选择仍在等待处理中。"}
	case phaseShop:
		if s.run != nil {
			return []string{fmt.Sprintf("商店已开启，当前可用金币 %d。", s.seatGoldLocked(selfID))}
		}
	case phaseRest:
		return []string{"篝火行动仍在等待处理中。"}
	case phaseEquipment:
		return []string{"装备替换提示仍在等待处理中。"}
	case phaseDeckAction:
		return []string{"牌组操作提示仍在等待处理中。"}
	case phaseSummary:
		return []string{"本局结算正在等待房主的下一步决定。"}
	}
	return nil
}

func (s *server) commandHintsLocked(selfID string) []string {
	return s.phasePresentationLocked(selfID, s.waitingOnForLocked(selfID)).Commands
}

func (s *server) roleNoteLocked(selfID string) string {
	return s.phasePresentationLocked(selfID, s.waitingOnForLocked(selfID)).RoleNote
}

func (s *server) controlLabelLocked(selfID string) string {
	return s.phasePresentationLocked(selfID, s.waitingOnForLocked(selfID)).ControlLabel
}

func (s *server) exampleCommandsLocked(selfID string) []string {
	return s.phasePresentationLocked(selfID, s.waitingOnForLocked(selfID)).Examples
}

func (s *server) phaseHintLocked(selfID string, waitingOn []string) string {
	return s.phasePresentationLocked(selfID, waitingOn).PhaseHint
}

func phaseHostWaitHint(isHost bool, hostText string) string {
	if isHost {
		return hostText
	}
	return "正在等待房主。"
}
