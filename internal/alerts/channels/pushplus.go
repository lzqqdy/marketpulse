package channels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const pushPlusURL = "https://www.pushplus.plus/send"

// PushPlus sends WeChat notifications via PushPlus one-to-one API.
type PushPlus struct {
	client *http.Client
}

func NewPushPlus() *PushPlus {
	return &PushPlus{client: &http.Client{Timeout: 15 * time.Second}}
}

type pushPlusReq struct {
	Token   string `json:"token"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type pushPlusResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (p *PushPlus) Send(token, title, content string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("pushplus token empty")
	}
	body, err := json.Marshal(pushPlusReq{Token: token, Title: title, Content: content})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, pushPlusURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("pushplus http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out pushPlusResp
	if err := json.Unmarshal(raw, &out); err == nil && out.Code != 200 {
		return fmt.Errorf("pushplus api: %s", out.Msg)
	}
	return nil
}
