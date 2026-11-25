package main

import (
	"strconv"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
)

// Importação de histórico: esperar a agenda antes de mandar ao Chatwoot.
//
// Ao parear, o WhatsApp manda o HISTÓRICO antes da AGENDA (app state
// critical_unblock_low). Importando na hora, os contatos nasciam no Chatwoot só
// com o número como nome. Agora os chunks do histórico ficam na fila (já foram
// gravados no banco por storeHistorySync) até a agenda chegar — ou até um tempo
// máximo, se ela demorar — e só então vão para o Chatwoot, já com os nomes.
//
// A equipe é avisada na conversa "AstraCalls (sistema)": quando a importação
// começa e quando termina (sem chunks novos por um tempo), com o total.
// Pedidos sob demanda (ON_DEMAND) não esperam: a agenda já está lá.

var (
	importAgendaWait = 3 * time.Minute  // tempo máximo esperando a agenda
	importIdleDone   = 90 * time.Second // sem chunks novos por este tempo = terminou
)

type importGate struct {
	mu        sync.Mutex
	ready     bool // agenda chegou (ou o tempo máximo passou)
	announced bool // aviso de início já enviado
	queue     []*waHistorySync.HistorySync
	waitTimer *time.Timer
	idleTimer *time.Timer
	convs     int
	msgs      int
	running   bool // um worker está processando a fila
}

var importGates sync.Map // sessão -> *importGate

// importChunk importa um chunk no Chatwoot (variável para os testes).
var importChunk = (*Session).importHistorySync

func (s *Session) importGate() *importGate {
	g, _ := importGates.LoadOrStore(s.id, &importGate{})
	return g.(*importGate)
}

// resetImportGate volta ao estado inicial (novo pareamento: a agenda virá de novo).
func (s *Session) resetImportGate() {
	g := s.importGate()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.waitTimer != nil {
		g.waitTimer.Stop()
	}
	if g.idleTimer != nil {
		g.idleTimer.Stop()
	}
	g.ready, g.announced, g.running = false, false, false
	g.queue, g.waitTimer, g.idleTimer = nil, nil, nil
	g.convs, g.msgs = 0, 0
}

// historyChunk recebe um chunk do HistorySync para importar no Chatwoot.
func (s *Session) historyChunk(data *waHistorySync.HistorySync) {
	if data == nil {
		return
	}
	cfg := s.getChatwoot()
	if !cfg.valid() || !cfg.ImportHistory || len(data.GetConversations()) == 0 {
		return
	}
	if data.GetSyncType() == waHistorySync.HistorySync_ON_DEMAND {
		s.runImportChunk(data)
		return
	}
	g := s.importGate()
	g.mu.Lock()
	if !g.announced {
		g.announced = true
		go s.chatwootAlert("⏳ *AstraCalls — Importación del historial*\n" +
			"La importación del número *" + s.displayName() + "* ya comenzó. " +
			"En unos minutos verás las conversaciones en esta bandeja (entran como resueltas y se reabren cuando el contacto vuelve a escribir).")
	}
	g.queue = append(g.queue, data)
	if !g.ready && g.waitTimer == nil {
		g.waitTimer = time.AfterFunc(importAgendaWait, func() {
			s.log.Info("chatwoot: histórico — agenda não chegou a tempo; importando assim mesmo")
			s.releaseImport()
		})
	}
	start := g.ready && !g.running
	if start {
		g.running = true
	}
	g.mu.Unlock()
	if start {
		s.drainImport()
	}
}

// releaseImport libera a fila (a agenda chegou ou o tempo acabou).
func (s *Session) releaseImport() {
	g := s.importGate()
	g.mu.Lock()
	g.ready = true
	if g.waitTimer != nil {
		g.waitTimer.Stop()
		g.waitTimer = nil
	}
	start := !g.running && len(g.queue) > 0
	if start {
		g.running = true
	}
	g.mu.Unlock()
	if start {
		s.drainImport()
	}
}

// drainImport importa os chunks da fila em ordem de chegada.
func (s *Session) drainImport() {
	g := s.importGate()
	for {
		g.mu.Lock()
		if len(g.queue) == 0 {
			g.running = false
			g.mu.Unlock()
			return
		}
		data := g.queue[0]
		g.queue = g.queue[1:]
		g.mu.Unlock()
		s.runImportChunk(data)
	}
}

// runImportChunk importa um chunk, soma ao total e (re)arma o aviso de fim.
func (s *Session) runImportChunk(data *waHistorySync.HistorySync) {
	convs, msgs := importChunk(s, data)
	g := s.importGate()
	g.mu.Lock()
	defer g.mu.Unlock()
	g.convs += convs
	g.msgs += msgs
	if g.idleTimer != nil {
		g.idleTimer.Stop()
	}
	g.idleTimer = time.AfterFunc(importIdleDone, s.announceImportDone)
}

func (s *Session) announceImportDone() {
	g := s.importGate()
	g.mu.Lock()
	if g.running || len(g.queue) > 0 || g.msgs == 0 {
		g.mu.Unlock()
		return
	}
	convs, msgs := g.convs, g.msgs
	g.convs, g.msgs = 0, 0
	g.mu.Unlock()
	s.chatwootAlert("✅ *AstraCalls — Importación terminada*\n" +
		"Número *" + s.displayName() + "*: " + strconv.Itoa(convs) + " conversaciones y " +
		strconv.Itoa(msgs) + " mensajes importados.")
}
