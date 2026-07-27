package admin

import "testing"

func TestDiffMissingInLive(t *testing.T) {
	live := map[string]any{"app": map[string]any{"mode": "debug"}}
	example := map[string]any{
		"app": map[string]any{"mode": "debug", "log_dir": "log"},
		"ai":  map[string]any{"enabled": false},
	}
	gaps := DiffTrees(live, example)
	kinds := map[string]string{}
	for _, g := range gaps {
		kinds[g.Path] = g.Kind
	}
	if kinds["app.log_dir"] != "missing_in_live" {
		t.Fatalf("app.log_dir=%q gaps=%+v", kinds["app.log_dir"], gaps)
	}
	if kinds["ai"] != "missing_in_live" {
		t.Fatalf("ai=%q", kinds["ai"])
	}
}

func TestDiffMissingInExample(t *testing.T) {
	live := map[string]any{"custom": 1, "app": map[string]any{"mode": "debug"}}
	example := map[string]any{"app": map[string]any{"mode": "debug"}}
	gaps := DiffTrees(live, example)
	found := false
	for _, g := range gaps {
		if g.Path == "custom" && g.Kind == "missing_in_example" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected custom missing_in_example, got %+v", gaps)
	}
}

func TestApplyGapValue(t *testing.T) {
	tree := map[string]any{"app": map[string]any{"mode": "debug"}}
	if err := ApplyGapValue(tree, "ai.enabled", true); err != nil {
		t.Fatal(err)
	}
	ai := tree["ai"].(map[string]any)
	if ai["enabled"] != true {
		t.Fatalf("%v", ai)
	}
}

func TestIsAdmin(t *testing.T) {
	dir := t.TempDir()
	adminFile := dir + "/admin.txt"
	cfg := dir + "/config.yaml"
	ex := dir + "/config.example.yaml"
	_ = writeFile(t, adminFile, "# comment\n13800138000\n")
	_ = writeFile(t, cfg, "app:\n  mode: debug\n  addr: \":8080\"\nsymbols: [BTC]\n")
	_ = writeFile(t, ex, "app:\n  mode: debug\n  addr: \":8080\"\n  log_dir: log\nsymbols: [BTC]\n")

	svc := New(adminFile, cfg, ex)
	if !svc.IsAdmin("13800138000") {
		t.Fatal("expected admin")
	}
	if svc.IsAdmin("13900000000") {
		t.Fatal("unexpected admin")
	}

	view, err := svc.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range view.Gaps {
		if g.Path == "app.log_dir" && g.Kind == "missing_in_live" {
			found = true
		}
	}
	if !found {
		t.Fatalf("gaps=%+v", view.Gaps)
	}
}

func writeFile(t *testing.T, path, body string) error {
	t.Helper()
	return atomicWriteFile(path, []byte(body))
}
