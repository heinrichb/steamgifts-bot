package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnabledDiscordOnly(t *testing.T) {
	n := New("https://discord.example.com/webhook", "", "")
	if !n.Enabled() {
		t.Error("expected enabled with discord URL")
	}
}

func TestEnabledTelegramOnly(t *testing.T) {
	n := New("", "bot-token", "chat-id")
	if !n.Enabled() {
		t.Error("expected enabled with telegram token+chat")
	}
}

func TestEnabledTelegramPartialDisabled(t *testing.T) {
	n := New("", "bot-token", "")
	if n.Enabled() {
		t.Error("telegram needs both token and chat to be enabled")
	}
}

func TestEnabledNoneConfigured(t *testing.T) {
	n := New("", "", "")
	if n.Enabled() {
		t.Error("expected disabled when nothing configured")
	}
}

func TestSendWinDiscord(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type: %s", ct)
		}
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	n := New(srv.URL, "", "")
	err := n.SendWin(context.Background(), Win{
		GameName:    "Half-Life 3",
		GiveawayURL: "https://www.steamgifts.com/giveaway/ABC/half-life-3",
		AccountName: "testuser",
	})
	if err != nil {
		t.Fatalf("SendWin: %v", err)
	}
	embeds, ok := gotBody["embeds"].([]any)
	if !ok || len(embeds) == 0 {
		t.Fatal("expected embeds in discord payload")
	}
	embed := embeds[0].(map[string]any)
	if !strings.Contains(embed["title"].(string), "Half-Life 3") {
		t.Errorf("embed title: %v", embed["title"])
	}
	if embed["url"] != "https://www.steamgifts.com/giveaway/ABC/half-life-3" {
		t.Errorf("embed url: %v", embed["url"])
	}
}

func TestSendWinDiscordNoURL(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	n := New(srv.URL, "", "")
	_ = n.SendWin(context.Background(), Win{GameName: "Game", AccountName: "user"})
	embeds := gotBody["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if _, hasURL := embed["url"]; hasURL {
		t.Error("embed should not have url when GiveawayURL is empty")
	}
}

func TestSendWinTelegram(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	origBase := telegramBaseURL
	telegramBaseURL = srv.URL
	defer func() { telegramBaseURL = origBase }()

	n := New("", "test-token", "12345")
	err := n.SendWin(context.Background(), Win{
		GameName:    "Portal 3",
		GiveawayURL: "https://example.com/giveaway",
		AccountName: "user1",
	})
	if err != nil {
		t.Fatalf("SendWin telegram: %v", err)
	}
	if gotPath != "/bottest-token/sendMessage" {
		t.Errorf("telegram path: %s", gotPath)
	}
	if gotBody["chat_id"] != "12345" {
		t.Errorf("chat_id: %v", gotBody["chat_id"])
	}
	text := gotBody["text"].(string)
	if !strings.Contains(text, "Portal 3") {
		t.Errorf("message should contain game name, got: %s", text)
	}
	if !strings.Contains(text, "user1") {
		t.Errorf("message should contain account name, got: %s", text)
	}
	if !strings.Contains(text, "example.com/giveaway") {
		t.Errorf("message should contain giveaway URL, got: %s", text)
	}
}

func TestSendWinTelegramNoGiveawayURL(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	origBase := telegramBaseURL
	telegramBaseURL = srv.URL
	defer func() { telegramBaseURL = origBase }()

	n := New("", "tok", "chat")
	_ = n.SendWin(context.Background(), Win{GameName: "Game", AccountName: "user"})
	text := gotBody["text"].(string)
	if strings.Contains(text, "View giveaway") {
		t.Error("should not include giveaway link when URL is empty")
	}
}

func TestSendWinBothTargets(t *testing.T) {
	discordCalled := false
	telegramCalled := false

	discordSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		discordCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discordSrv.Close()

	telegramSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		telegramCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer telegramSrv.Close()

	origBase := telegramBaseURL
	telegramBaseURL = telegramSrv.URL
	defer func() { telegramBaseURL = origBase }()

	n := New(discordSrv.URL, "tok", "chat")
	err := n.SendWin(context.Background(), Win{GameName: "G", AccountName: "u"})
	if err != nil {
		t.Fatalf("SendWin: %v", err)
	}
	if !discordCalled {
		t.Error("discord webhook not called")
	}
	if !telegramCalled {
		t.Error("telegram API not called")
	}
}

func TestSendWinServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := New(srv.URL, "", "")
	err := n.SendWin(context.Background(), Win{GameName: "Game", AccountName: "user"})
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected status 500 in error, got: %v", err)
	}
}

func TestSendWinDiscordErrorDoesNotBlockTelegram(t *testing.T) {
	discordSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer discordSrv.Close()

	telegramCalled := false
	telegramSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		telegramCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer telegramSrv.Close()

	origBase := telegramBaseURL
	telegramBaseURL = telegramSrv.URL
	defer func() { telegramBaseURL = origBase }()

	n := New(discordSrv.URL, "tok", "chat")
	err := n.SendWin(context.Background(), Win{GameName: "G", AccountName: "u"})
	// Discord fails, so we should get an error...
	if err == nil {
		t.Fatal("expected error from failed discord")
	}
	// ...but telegram should still have been attempted.
	if !telegramCalled {
		t.Error("telegram should be called even when discord fails")
	}
}

func TestSendWinNoTargets(t *testing.T) {
	n := New("", "", "")
	err := n.SendWin(context.Background(), Win{GameName: "G", AccountName: "u"})
	if err != nil {
		t.Fatalf("expected no error with no targets, got: %v", err)
	}
}

func TestSendWinContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n := New(srv.URL, "", "")
	err := n.SendWin(ctx, Win{GameName: "G", AccountName: "u"})
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestPostJSONMarshalError(t *testing.T) {
	n := New("", "", "")
	err := n.postJSON(context.Background(), "http://example.com", map[string]any{"bad": make(chan int)}, "test")
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "marshal") {
		t.Errorf("expected marshal error, got: %v", err)
	}
}
