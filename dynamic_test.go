package mks

import (
	"slices"
	"sync"
	"testing"
)

type dynItem struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func TestSetResources(t *testing.T) {
	p := &Panel{resources: map[string]registered{}}
	p.Register(Define[dynItem](Config{Name: "posts", Key: "id"}))

	if err := p.SetResources("plugin:b", Define[dynItem](Config{Name: "entries_b", Key: "id"})); err != nil {
		t.Fatal(err)
	}
	if err := p.SetResources("plugin:a", Define[dynItem](Config{Name: "entries_a1", Key: "id"}), Define[dynItem](Config{Name: "entries_a2", Key: "id"})); err != nil {
		t.Fatal(err)
	}
	// Start resources first, then the groups in name order.
	if got := p.names(); !slices.Equal(got, []string{"posts", "entries_a1", "entries_a2", "entries_b"}) {
		t.Fatalf("names = %v", got)
	}
	if !p.hasResource("entries_a2") || p.mustLookup("entries_b").cfg.Name != "entries_b" {
		t.Fatal("added resources are not found")
	}

	// A group's resources are replaced as a whole.
	if err := p.SetResources("plugin:a", Define[dynItem](Config{Name: "entries_a3", Key: "id"})); err != nil {
		t.Fatal(err)
	}
	if p.hasResource("entries_a1") || !p.hasResource("entries_a3") || !p.hasResource("entries_b") {
		t.Fatalf("after replacing: %v", p.names())
	}

	// Names never clash, with start resources or another group; a bad config is refused
	// and changes nothing.
	for _, r := range []Resource{Define[dynItem](Config{Name: "posts", Key: "id"}), Define[dynItem](Config{Name: "entries_b", Key: "id"}), Define[dynItem](Config{Name: "", Key: "id"})} {
		if err := p.SetResources("plugin:c", r); err == nil {
			t.Errorf("%q was accepted", r.Config().Name)
		}
	}
	if p.hasResource("") || len(p.names()) != 3 {
		t.Fatalf("a refused set changed the panel: %v", p.names())
	}

	// Removing a group: no resources.
	if err := p.SetResources("plugin:a"); err != nil || p.hasResource("entries_a3") {
		t.Fatalf("group not removed: %v %v", err, p.names())
	}

	// Readers and a writer at once (run with -race).
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				_ = p.SetResources("plugin:z", Define[dynItem](Config{Name: "entries_z", Key: "id"}))
			} else {
				for _, n := range p.names() {
					p.lookup(n)
				}
			}
		}()
	}
	wg.Wait()
}
