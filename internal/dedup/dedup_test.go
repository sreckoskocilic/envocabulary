package dedup

import (
	"reflect"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

func TestFind(t *testing.T) {
	files := []inventory.File{
		{Path: "/z/.zprofile", Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "JAVA_HOME", Line: 10},
			{Kind: inventory.KindExport, Name: "EDITOR", Line: 11},
			{Kind: inventory.KindExport, Name: "GOPATH", Line: 15},
			{Kind: inventory.KindExport, Name: "GOPATH", Line: 20},
		}},
		{Path: "/z/.zshrc", Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "JAVA_HOME", Line: 5},
			{Kind: inventory.KindAlias, Name: "ll", Line: 7},
			{Kind: inventory.KindSource, Name: "~/.zprofile", Line: 9},
		}},
	}

	groups := Find(files)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d: %+v", len(groups), groups)
	}

	var javaGroup, gopathGroup *Group
	for i := range groups {
		switch groups[i].Name {
		case "JAVA_HOME":
			javaGroup = &groups[i]
		case "GOPATH":
			gopathGroup = &groups[i]
		}
	}
	if javaGroup == nil || gopathGroup == nil {
		t.Fatalf("missing expected group: %+v", groups)
	}

	wantJavaWinner := Occurrence{File: "/z/.zshrc", Kind: inventory.KindExport, Name: "JAVA_HOME", Line: 5}
	if !reflect.DeepEqual(javaGroup.Winner, wantJavaWinner) {
		t.Errorf("JAVA_HOME winner got %+v want %+v", javaGroup.Winner, wantJavaWinner)
	}
	if len(javaGroup.Losers) != 1 || javaGroup.Losers[0].File != "/z/.zprofile" || javaGroup.Losers[0].Line != 10 {
		t.Errorf("JAVA_HOME losers got %+v", javaGroup.Losers)
	}

	if gopathGroup.Winner.Line != 20 {
		t.Errorf("GOPATH winner line got %d want 20", gopathGroup.Winner.Line)
	}
	if len(gopathGroup.Losers) != 1 || gopathGroup.Losers[0].Line != 15 {
		t.Errorf("GOPATH losers got %+v", gopathGroup.Losers)
	}
}

func TestFindIgnoresSources(t *testing.T) {
	files := []inventory.File{
		{Path: "/a", Items: []inventory.Item{{Kind: inventory.KindSource, Name: "foo", Line: 1}}},
		{Path: "/b", Items: []inventory.Item{{Kind: inventory.KindSource, Name: "foo", Line: 1}}},
	}
	if groups := Find(files); len(groups) != 0 {
		t.Errorf("expected sources to be excluded from dedup, got %+v", groups)
	}
}

func TestFindExcludesDeferredListVars(t *testing.T) {
	files := []inventory.File{
		{Path: "/a", Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "PATH", Line: 1},
			{Kind: inventory.KindExport, Name: "MANPATH", Line: 2},
			{Kind: inventory.KindExport, Name: "DYLD_LIBRARY_PATH", Line: 3},
		}},
		{Path: "/b", Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "PATH", Line: 1},
			{Kind: inventory.KindExport, Name: "MANPATH", Line: 2},
			{Kind: inventory.KindExport, Name: "DYLD_LIBRARY_PATH", Line: 3},
		}},
	}
	if groups := Find(files); len(groups) != 0 {
		t.Errorf("expected deferred-list-vars to be excluded, got %+v", groups)
	}
}

func TestFind_MixedKindsSortCorrectly(t *testing.T) {
	files := []inventory.File{
		{Path: "/a", Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "FOO", Line: 1},
			{Kind: inventory.KindAlias, Name: "ll", Line: 2},
		}},
		{Path: "/b", Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "FOO", Line: 1},
			{Kind: inventory.KindAlias, Name: "ll", Line: 2},
		}},
	}
	groups := Find(files)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Kind != inventory.KindAlias || groups[1].Kind != inventory.KindExport {
		t.Errorf("expected alias before export; got %s, %s", groups[0].Kind, groups[1].Kind)
	}
}

func TestFindNoDuplicatesReturnsEmpty(t *testing.T) {
	files := []inventory.File{
		{Path: "/a", Items: []inventory.Item{{Kind: inventory.KindExport, Name: "UNIQUE", Line: 1}}},
	}
	if groups := Find(files); len(groups) != 0 {
		t.Errorf("expected no groups, got %+v", groups)
	}
}

func TestLoserSet(t *testing.T) {
	groups := []Group{
		{
			Kind: inventory.KindExport, Name: "X",
			Winner: Occurrence{File: "/b", Line: 5},
			Losers: []Occurrence{{File: "/a", Line: 10}, {File: "/a", Line: 20}},
		},
	}
	set := LoserSet(groups)
	if len(set) != 2 {
		t.Errorf("expected 2 loser entries, got %d", len(set))
	}
	if _, ok := set[Key("/a", 10)]; !ok {
		t.Errorf("expected /a:10 in loser set")
	}
	if _, ok := set[Key("/a", 20)]; !ok {
		t.Errorf("expected /a:20 in loser set")
	}
}
