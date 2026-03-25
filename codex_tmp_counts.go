package main
import (
  "fmt"
  "cmdcards/internal/content"
)
func main() {
  lib, err := content.LoadEmbedded()
  if err != nil { panic(err) }
  fmt.Printf("cards=%d relics=%d equipments=%d\n", len(lib.CardList()), len(lib.RelicList()), len(lib.EquipmentList()))
}
