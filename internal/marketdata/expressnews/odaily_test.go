package expressnews

import "testing"

func TestNormalizeOdailyItem(t *testing.T) {
	item := normalizeOdailyItem(odailyNewsItem{
		ID:               502991,
		Title:            "英国计划发行代币化主权债券",
		Content:          "<p>Odaily星球日报讯 英国计划发行<strong>代币化</strong>主权债券。</p>",
		IsImportant:      true,
		PublishTimestamp: 1784734730000,
		SourceURL:        "https://example.com/src",
		Link:             "https://www.odaily.news/zh-CN/newsflash/502991",
	})
	if item.ID != "odaily:502991" {
		t.Fatalf("id=%s", item.ID)
	}
	if !item.Important {
		t.Fatal("expected important")
	}
	if item.PublishTime != 1784734730 {
		t.Fatalf("publish=%d", item.PublishTime)
	}
	if item.Provider != "Odaily" {
		t.Fatalf("provider=%s", item.Provider)
	}
	if item.Tag != TagCrypto {
		t.Fatalf("tag=%s", item.Tag)
	}
	if item.Body != "Odaily星球日报讯 英国计划发行代币化主权债券。" {
		t.Fatalf("body=%q", item.Body)
	}
	if item.ThirdURL != "https://www.odaily.news/zh-CN/newsflash/502991" {
		t.Fatalf("url=%s", item.ThirdURL)
	}
}

func TestMsToUnixSeconds(t *testing.T) {
	if got := msToUnixSeconds(1784734730000); got != 1784734730 {
		t.Fatalf("ms got=%d", got)
	}
	if got := msToUnixSeconds(1784734730); got != 1784734730 {
		t.Fatalf("sec got=%d", got)
	}
}

func TestStripHTML(t *testing.T) {
	got := stripHTML(`<p>Hello&nbsp;<b>World</b></p>`)
	if got != "Hello World" {
		t.Fatalf("got=%q", got)
	}
}
