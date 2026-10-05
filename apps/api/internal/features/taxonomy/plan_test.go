package taxonomy

import (
	"testing"
)

func planFixture() graph {
	return graph{Catalog: Catalog{Revision: "revision", Tags: []Tag{
		{ID: "p1", Name: "First", Enabled: true}, {ID: "p2", Name: "Second", Enabled: true},
		{ID: "s1", Name: "Child1", Enabled: true}, {ID: "s2", Name: "Child2", Enabled: true},
		{ID: "t1", Name: "Tag1", Enabled: true}, {ID: "t2", Name: "Tag2", Enabled: true},
	}, Cascades: []Cascade{
		{ID: "c1", Scope: "SITE", PrimaryID: "p1", SecondaryID: "s1", Enabled: true}, {ID: "c2", Scope: "SITE", PrimaryID: "p2", SecondaryID: "s2", Enabled: true},
		{ID: "a1", Scope: "ARTICLE", PrimaryID: "p1", SecondaryID: "s1", Enabled: true}, {ID: "a2", Scope: "ARTICLE", PrimaryID: "p2", SecondaryID: "s2", Enabled: true},
	}}, Objects: []object{{ID: "site", Scope: "SITE", CascadeID: "c1", Tags: []assignment{{TagID: "t1", Role: "TERTIARY", Position: 1}, {TagID: "t2", Role: "TERTIARY", Position: 2}}}}}
}

func TestMergeDeduplicatesAndPreservesEarliestPosition(t *testing.T) {
	p, err := buildPlan(planFixture(), ChangeInput{Kind: "merge", SourceID: "t1", TargetID: "t2"})
	if err != nil || len(p.Blockers) > 0 {
		t.Fatalf("plan %v %v", p.Blockers, err)
	}
	tags := p.Graph.Objects[0].Tags
	if len(tags) != 1 || tags[0].TagID != "t2" || tags[0].Position != 1 || p.RemovedDuplicates != 1 || p.SiteCount != 1 {
		t.Fatalf("incorrect merge %#v", p)
	}
}
func TestMergeRejectsWarningConflict(t *testing.T) {
	g := planFixture()
	g.Objects[0].Tags[1].Role = "WARNING"
	g.Objects[0].Tags[1].Position = 0
	p, err := buildPlan(g, ChangeInput{Kind: "merge", SourceID: "t1", TargetID: "t2"})
	if err != nil || len(p.Blockers) == 0 {
		t.Fatalf("warning conflict not blocked: %#v %v", p, err)
	}
}
func TestMergeUpdatesBothEndpointsAndScopes(t *testing.T) {
	g := planFixture()
	g.Cascades = append(g.Cascades, Cascade{ID: "self", Scope: "SITE", PrimaryID: "s1", SecondaryID: "s1", Enabled: true})
	p, err := buildPlan(g, ChangeInput{Kind: "merge", SourceID: "s1", TargetID: "p2"})
	if err != nil || len(p.Blockers) > 0 {
		t.Fatal(err, p.Blockers)
	}
	if findCascade(p.Graph, "c1").SecondaryID != "p2" || findCascade(p.Graph, "a1").SecondaryID != "p2" || findCascade(p.Graph, "self").PrimaryID != "p2" || findCascade(p.Graph, "self").SecondaryID != "p2" {
		t.Fatal("merge failed to rewrite both roles")
	}
	if findTag(p.Graph, "s1").ID != "" {
		t.Fatal("source remained in dictionary")
	}
}
func TestPathUpdateAllowsSelfRelation(t *testing.T) {
	p, err := buildPlan(planFixture(), ChangeInput{Kind: "path_update", SourceID: "c1", PrimaryID: "p1", SecondaryID: "p1", Enabled: true})
	if err != nil || len(p.Blockers) > 0 || findCascade(p.Graph, "c1").SecondaryID != "p1" || p.SiteCount != 1 {
		t.Fatalf("self path %#v %v", p, err)
	}
	if findCascade(p.Graph, "a1").SecondaryID != "s1" {
		t.Fatal("path edit changed unrelated scope")
	}
}
func TestPathMergeRedirectsObjects(t *testing.T) {
	p, err := buildPlan(planFixture(), ChangeInput{Kind: "path_merge", SourceID: "c1", TargetID: "c2"})
	if err != nil || len(p.Blockers) > 0 || p.Graph.Objects[0].CascadeID != "c2" {
		t.Fatalf("path merge %#v %v", p, err)
	}
}
func TestMergeRejectsSourceMetadataConflict(t *testing.T) {
	g := planFixture()
	g.Objects[0].Tags[0].Source = "MANUAL"
	g.Objects[0].Tags[1].Source = "IMPORTED"
	p, err := buildPlan(g, ChangeInput{Kind: "merge", SourceID: "t1", TargetID: "t2"})
	if err != nil || len(p.Blockers) == 0 {
		t.Fatal("source conflict lost")
	}
}
func TestMergeCoalescesPaths(t *testing.T) {
	g := planFixture()
	g.Cascades[1].PrimaryID = "p1"
	p, err := buildPlan(g, ChangeInput{Kind: "merge", SourceID: "s2", TargetID: "s1"})
	if err != nil || len(p.Blockers) > 0 || findCascade(p.Graph, "c2").MergedIntoID != "c1" {
		t.Fatalf("coalescing %#v %v", p, err)
	}
}
func TestFingerprintBindsInputAndRevision(t *testing.T) {
	g := planFixture()
	input := ChangeInput{Kind: "merge", SourceID: "t1", TargetID: "t2"}
	first, _ := buildPlan(g, input)
	input.Fingerprint = first.Fingerprint
	same, _ := buildPlan(g, input)
	if same.Fingerprint != first.Fingerprint {
		t.Fatal("fingerprint depends on itself")
	}
	g.Revision = "changed"
	second, _ := buildPlan(g, input)
	if second.Fingerprint == first.Fingerprint {
		t.Fatal("fingerprint ignores revision")
	}
}
