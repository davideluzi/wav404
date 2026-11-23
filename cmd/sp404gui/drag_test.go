package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

// center is the absolute canvas position of the middle of a tile.
func center(t *padTile) fyne.Position {
	p := fyne.CurrentApp().Driver().AbsolutePositionForObject(t)
	return p.Add(fyne.NewPos(t.Size().Width/2, t.Size().Height/2))
}

// flagged lists the tiles showing the drop highlight.
func flagged(g *gui) []int {
	var out []int
	for i, t := range g.tiles {
		if t.dropTarget {
			out = append(out, i)
		}
	}
	return out
}

// The highlighted pad is the pad a drop at the same position imports into.
func TestDragOverMatchesDropTarget(t *testing.T) {
	g, _ := newTestGUI(t)
	pos := center(g.tiles[4]) // A5
	g.dragOver(pos)
	if got := flagged(g); len(got) != 1 || got[0] != 4 {
		t.Fatalf("highlighted tiles = %v, want [4]", got)
	}
	g.drop(pos, []fyne.URI{storage.NewFileURI(fixture("s24.wav"))})
	if g.card.Info.Pads[4].Empty() {
		t.Error("drop did not import into the highlighted pad A5")
	}
	if got := flagged(g); len(got) != 0 {
		t.Errorf("highlight left after drop: %v", got)
	}
}

func TestDragOutsidePadsClearsHighlight(t *testing.T) {
	g, _ := newTestGUI(t)
	g.dragOver(center(g.tiles[2]))
	g.dragOver(fyne.NewPos(1, 1)) // toolbar corner
	if got := flagged(g); len(got) != 0 {
		t.Errorf("highlighted tiles = %v, want none", got)
	}
}

func TestDragGoneClearsHighlight(t *testing.T) {
	g, _ := newTestGUI(t)
	g.dragOver(center(g.tiles[2]))
	g.dragGone()
	if got := flagged(g); len(got) != 0 {
		t.Errorf("highlighted tiles = %v, want none", got)
	}
}

func TestBankChangeClearsDropTarget(t *testing.T) {
	g, _ := newTestGUI(t)
	g.dragOver(center(g.tiles[2]))
	g.banks.SetSelected("B")
	if got := flagged(g); len(got) != 0 {
		t.Errorf("highlighted tiles after bank change = %v", got)
	}
}

func TestDragWithoutCardHighlightsNothing(t *testing.T) {
	g, _ := newTestGUI(t)
	g.card = nil
	g.dragOver(center(g.tiles[2]))
	if got := flagged(g); len(got) != 0 {
		t.Errorf("highlighted tiles without card = %v", got)
	}
}
