package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func (a *API) registerChatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/claude/status", a.handleClaudeStatus)
	mux.HandleFunc("GET /api/conversations", a.handleListConversations)
	mux.HandleFunc("POST /api/conversations", a.handleCreateConversation)
	mux.HandleFunc("GET /api/conversations/{id}", a.handleGetConversation)
	mux.HandleFunc("PATCH /api/conversations/{id}", a.handleUpdateConversation)
	mux.HandleFunc("DELETE /api/conversations/{id}", a.handleDeleteConversation)
	mux.HandleFunc("GET /api/conversations/{id}/messages", a.handleListMessages)
	mux.HandleFunc("POST /api/conversations/{id}/messages", a.handleSendMessage)
	mux.HandleFunc("GET /api/conversations/{id}/stream", a.handleConversationStream)
	mux.HandleFunc("POST /api/conversations/{id}/stop", a.handleStopConversation)
}

func writeChatError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrConversationNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (a *API) claudeStatus(ctx context.Context) map[string]any {
	path, err := exec.LookPath(a.claudeBin)
	if err != nil {
		return map[string]any{"available": false}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return map[string]any{"available": false, "path": path, "error": err.Error()}
	}
	return map[string]any{"available": true, "path": path, "version": strings.TrimSpace(string(out))}
}

func (a *API) handleClaudeStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.claudeStatus(r.Context()))
}

func (a *API) handleListConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := a.chat.List()
	if err != nil {
		writeChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, convs)
}

func (a *API) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	var req CreateConversationRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Model == "" {
		req.Model = a.llmSettings().DefaultModel
	}
	c, err := a.chat.Create(req)
	if err != nil {
		writeChatError(w, err)
		return
	}
	a.Broadcast("conversation_created", c)
	writeJSON(w, http.StatusCreated, c)
}

func (a *API) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	c, err := a.chat.Get(r.PathValue("id"))
	if err != nil {
		writeChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *API) handleUpdateConversation(w http.ResponseWriter, r *http.Request) {
	var req UpdateConversationRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	c, err := a.chat.Update(r.PathValue("id"), func(c *Conversation) {
		if req.Title != nil {
			c.Title = *req.Title
		}
		if req.Model != nil {
			c.Model = *req.Model
		}
		if req.Agent != nil {
			c.Agent = *req.Agent
		}
	})
	if err != nil {
		writeChatError(w, err)
		return
	}
	a.Broadcast("conversation_updated", c)
	writeJSON(w, http.StatusOK, c)
}

func (a *API) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := a.chat.Get(id); err != nil {
		writeChatError(w, err)
		return
	}
	if a.runner != nil {
		a.runner.Stop(id) //nolint:errcheck
	}
	if err := a.chat.Delete(id); err != nil {
		writeChatError(w, err)
		return
	}
	a.Broadcast("conversation_deleted", map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleListMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := a.chat.Get(id); err != nil {
		writeChatError(w, err)
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	events, err := a.chat.Events(id, after)
	if err != nil {
		writeChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (a *API) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req SendMessageRequest
	if err := decodeBody(r, &req); err != nil || strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	conv, err := a.chat.Get(id)
	if err != nil {
		writeChatError(w, err)
		return
	}
	if a.runner == nil || !a.runner.Available() {
		writeError(w, http.StatusServiceUnavailable, "claude binary not found")
		return
	}
	if !a.runner.Running(id) {
		if err := a.runner.Start(r.Context(), id, conv.Model, conv.Agent, conv.SystemPrompt); err != nil {
			if errors.Is(err, ErrRunnerAtCapacity) {
				writeError(w, http.StatusTooManyRequests, err.Error())
				return
			}
			writeChatError(w, err)
			return
		}
	}
	if conv.Title == "New conversation" {
		title := strings.TrimSpace(req.Message)
		if len(title) > 60 {
			title = title[:57] + "..."
		}
		if updated, err := a.chat.Update(id, func(c *Conversation) { c.Title = title }); err == nil {
			a.Broadcast("conversation_updated", updated)
		}
	}
	ev, err := a.chat.AppendEvent(id, ChatEvent{Kind: ChatKindUser, Text: req.Message})
	if err != nil {
		writeChatError(w, err)
		return
	}
	if payload, err := json.Marshal(ev); err == nil {
		a.hub.Publish(chatTopic(id), payload)
	}
	if err := a.runner.Send(id, req.Message); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, ev)
}

func (a *API) handleStopConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := a.chat.Get(id); err != nil {
		writeChatError(w, err)
		return
	}
	if a.runner != nil {
		if err := a.runner.Stop(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleConversationStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := a.chat.Get(id); err != nil {
		writeChatError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsubscribe := a.hub.Subscribe(chatTopic(id))
	defer unsubscribe()

	w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	var lastSeq int64
	if h := r.Header.Get("Last-Event-ID"); h != "" {
		lastSeq, _ = strconv.ParseInt(h, 10, 64)
		backfill, err := a.chat.Events(id, lastSeq)
		if err != nil {
			return
		}
		for _, ev := range backfill {
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			writeSSEEvent(w, ev.Seq, payload)
			lastSeq = ev.Seq
		}
		flusher.Flush()
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			var head struct {
				Seq int64 `json:"seq"`
			}
			if json.Unmarshal(msg, &head) != nil || head.Seq <= lastSeq {
				continue
			}
			lastSeq = head.Seq
			writeSSEEvent(w, head.Seq, msg)
			flusher.Flush()
		case <-ticker.C:
			w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, seq int64, payload []byte) {
	w.Write([]byte("id: " + strconv.FormatInt(seq, 10) + "\ndata: "))
	w.Write(payload)
	w.Write([]byte("\n\n"))
}
