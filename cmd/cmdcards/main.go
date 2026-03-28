package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"cmdcards/internal/app"
	"cmdcards/internal/content"
	"cmdcards/internal/engine"
	"cmdcards/internal/netplay"
	"cmdcards/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		args = []string{"play"}
	}

	lib, err := content.LoadEmbedded()
	if err != nil {
		return err
	}

	store, err := storage.NewDefaultStore()
	if err != nil {
		return err
	}

	switch args[0] {
	case "play":
		return app.Run(lib, store)
	case "validate-content":
		fmt.Printf("内容校验通过: 职业 %d, 卡牌 %d, 遗物 %d, 药水 %d, 装备 %d, 敌人遭遇 %d, 事件 %d\n",
			len(lib.Classes), len(lib.Cards), len(lib.Relics), len(lib.Potions), len(lib.Equipments), len(lib.Encounters), len(lib.Events))
		return nil
	case "export-wiki":
		return runExportWiki(lib, args[1:])
	case "smoke":
		return runSmoke(lib, args[1:])
	case "host":
		return runHost(lib, args[1:])
	case "join":
		return runJoin(lib, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runSmoke(lib *content.Library, args []string) error {
	mode := engine.ModeStory
	classID := "vanguard"
	seed := time.Now().UnixNano()

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mode":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --mode")
			}
			switch strings.ToLower(args[i]) {
			case "story":
				mode = engine.ModeStory
			case "endless":
				mode = engine.ModeEndless
			default:
				return fmt.Errorf("invalid mode %q", args[i])
			}
		case "--class":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --class")
			}
			classID = args[i]
		case "--seed":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --seed")
			}
			parsed, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return fmt.Errorf("parse seed: %w", err)
			}
			seed = parsed
		default:
			return fmt.Errorf("unknown smoke flag %q", args[i])
		}
	}

	profile := engine.DefaultProfile(lib)
	result, err := engine.RunSmoke(lib, profile, mode, classID, seed)
	if err != nil {
		return err
	}

	fmt.Printf("Smoke complete: mode=%s class=%s seed=%d result=%s acts=%d floors=%d hp=%d gold=%d deck=%d wins=%d\n",
		result.Mode, result.ClassID, result.Seed, result.Result, result.ReachedAct, result.ClearedFloors, result.FinalHP, result.FinalGold, result.FinalDeckSize, result.CombatsWon)
	for _, line := range result.Log {
		fmt.Println("-", line)
	}
	return nil
}

func runHost(lib *content.Library, args []string) error {
	port := 7777
	name := "Host"
	classID := "vanguard"
	forceNew := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --port")
			}
			parsed, err := strconv.Atoi(args[i])
			if err != nil {
				return fmt.Errorf("parse port: %w", err)
			}
			port = parsed
		case "--name":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --name")
			}
			name = args[i]
		case "--class":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --class")
			}
			classID = args[i]
		case "--new":
			forceNew = true
		default:
			return fmt.Errorf("unknown host flag %q", args[i])
		}
	}

	if _, ok := lib.Classes[classID]; !ok {
		return fmt.Errorf("unknown class %q", classID)
	}
	return netplay.RunHost(lib, port, name, classID, forceNew)
}

func runJoin(lib *content.Library, args []string) error {
	addr := ""
	name := "Guest"
	classID := "arcanist"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --addr")
			}
			addr = args[i]
		case "--name":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --name")
			}
			name = args[i]
		case "--class":
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for --class")
			}
			classID = args[i]
		default:
			return fmt.Errorf("unknown join flag %q", args[i])
		}
	}

	if addr == "" {
		return fmt.Errorf("missing required flag --addr")
	}
	if _, ok := lib.Classes[classID]; !ok {
		return fmt.Errorf("unknown class %q", classID)
	}
	return netplay.RunJoin(lib, addr, name, classID)
}
