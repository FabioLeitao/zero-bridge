package a2a

import "testing"

func TestDefaultAgentCardUsesAgentName(t *testing.T) {
	card := DefaultAgentCard("acptest", "0")
	if card.Name != "acptest" || card.Version != "0" {
		t.Fatalf("card %+v", card)
	}
	if card.Description == "" || len(card.Capabilities) == 0 || len(card.Skills) == 0 {
		t.Fatalf("card %+v", card)
	}
	if card.Skills[0].ID != "chat" {
		t.Fatalf("skill %+v", card.Skills[0])
	}
}

func TestDefaultAgentCardFallsBackWhenNameEmpty(t *testing.T) {
	card := DefaultAgentCard("", "1")
	if card.Name != "zero-bridge" || card.Version != "1" {
		t.Fatalf("card %+v", card)
	}
}
