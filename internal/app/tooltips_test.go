package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func hover(t *testing.T, tt *ui.Tester, text string) {
	t.Helper()
	r, ok := tt.Find(text)
	if !ok {
		t.Fatalf("no %q on screen: %q", text, tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Advance(700 * time.Millisecond)
	tt.Frame()
}

func TestHoverExplainsHintBarActions(t *testing.T) {
	a, tt, _ := newTestApp(t)
	tr := a.repo.Common.Tr
	press(tt, "2")
	if tt.HasText(tr.StageTooltip) {
		t.Fatal("tooltip shows before any hover")
	}
	hover(t, tt, tr.Stage)
	if !tt.HasText(tr.StageTooltip) {
		t.Fatalf("hovering the files panel's %q shows no %q: %q", tr.Stage, tr.StageTooltip, tt.Texts())
	}
	tt.Move(5, 5)
	tt.Advance(time.Second)
	tt.Frame()
	if tt.HasText(tr.StageTooltip) {
		t.Fatal("tooltip stays after the pointer leaves")
	}
}

func TestHoverExplainsGuideButtons(t *testing.T) {
	a, tt, _ := newTestApp(t)
	press(tt, "~")
	tt.Frame()
	defer a.closeGuide()
	hover(t, tt, "Working tree · b")
	if !tt.HasText(guideTooltips["Base"]) {
		t.Fatalf("hovering the base button shows no tooltip: %q", tt.Texts())
	}
	hover(t, tt, "A ask")
	if !tt.HasText(guideTooltips["Copy agent prompt"]) {
		t.Fatalf("hovering the ask step shows no tooltip: %q", tt.Texts())
	}
	if err := tt.Click("Working tree · b"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Review branch against base") {
		t.Fatalf("the base button no longer runs its action: %q", tt.Texts())
	}
}

func TestGuideTooltipsNameRealActions(t *testing.T) {
	a, _, _ := newTestApp(t)
	for label := range guideTooltips {
		if a.guideAct(label).run == nil {
			t.Errorf("guideTooltips[%q] names no review guide action", label)
		}
	}
}
