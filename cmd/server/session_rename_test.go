package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCleanSessionName(t *testing.T) {
	if got := cleanSessionName("  AstraCalls   0867 \n"); got != "AstraCalls 0867" {
		t.Fatalf("got %q", got)
	}
	if cleanSessionName("   ") != "" {
		t.Fatal("só espaços = vazio")
	}
	long := strings.Repeat("ñ", 80)
	if got := cleanSessionName(long); len([]rune(got)) != maxSessionName {
		t.Fatalf("corta em %d caracteres, veio %d", maxSessionName, len([]rune(got)))
	}
}

func TestRenameSession(t *testing.T) {
	m := &SessionManager{sessions: map[string]*Session{}}
	s := &Session{id: "s1", name: "Prueba Provisioner", mgr: m}
	m.sessions["s1"] = s
	if err := m.Rename(context.Background(), "s1", "AstraCalls 0867"); err != nil {
		t.Fatal(err)
	}
	if s.displayName() != "AstraCalls 0867" {
		t.Fatalf("nome = %q", s.displayName())
	}
	if err := m.Rename(context.Background(), "nao-existe", "x"); err == nil {
		t.Fatal("sessão inexistente deve falhar")
	}
	// sessão do AstraChat ainda sem Chatwoot: o nome é o escopo do QR
	scoped := &Session{id: "s2", name: "inbox38_acc1", mgr: m}
	m.sessions["s2"] = scoped
	if err := m.Rename(context.Background(), "s2", "Outro"); !errors.Is(err, errRenameScoped) {
		t.Fatalf("esperava errRenameScoped, veio %v", err)
	}
	scoped.setChatwoot(ChatwootConfig{URL: "https://cw", AccountID: 1, AccountToken: "t", InboxID: 38})
	if err := m.Rename(context.Background(), "s2", "Outro"); err != nil {
		t.Fatalf("com Chatwoot configurado pode renomear: %v", err)
	}
}
