package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"zero-bridge/bridge/internal/sessionmeta"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
	maxQueryRunes    = 100
)

// listACPPageCap is how many session/list calls one HTTP request may make.
// Tests lower it to prove the truncated flag.
var listACPPageCap = 20

type listFilter struct {
	query    string
	archived string
	limit    int
}

type listCursor struct {
	ACP string `json:"a,omitempty"`
	At  int    `json:"i,omitempty"`
}

type listPage struct {
	sessions  []json.RawMessage
	next      string
	truncated bool
}

func parseListFilter(q, archived, limitRaw string) (listFilter, error) {
	if utf8.RuneCountInString(q) > maxQueryRunes {
		return listFilter{}, fmt.Errorf("q is too long")
	}
	switch archived {
	case "", "exclude", "include", "only":
	default:
		return listFilter{}, fmt.Errorf("archived must be exclude, include, or only")
	}
	if archived == "" {
		archived = "exclude"
	}
	limit := defaultListLimit
	if limitRaw != "" {
		n, err := parsePositiveLimit(limitRaw)
		if err != nil {
			return listFilter{}, err
		}
		limit = n
	}
	return listFilter{query: q, archived: archived, limit: limit}, nil
}

func parsePositiveLimit(raw string) (int, error) {
	n := 0
	if raw == "" {
		return 0, fmt.Errorf("limit must be from 1 to %d", maxListLimit)
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("limit must be from 1 to %d", maxListLimit)
		}
		n = n*10 + int(r-'0')
		if n > maxListLimit {
			return 0, fmt.Errorf("limit must be from 1 to %d", maxListLimit)
		}
	}
	if n < 1 {
		return 0, fmt.Errorf("limit must be from 1 to %d", maxListLimit)
	}
	return n, nil
}

func decodeListCursor(raw string) (listCursor, error) {
	if raw == "" {
		return listCursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return listCursor{}, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	var cur listCursor
	if err := dec.Decode(&cur); err != nil || cur.At < 0 {
		return listCursor{}, fmt.Errorf("invalid cursor")
	}
	return cur, nil
}

func encodeListCursor(cur listCursor) string {
	b, err := json.Marshal(cur)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *Server) collectSessions(ctx context.Context, cwd string, start listCursor, filter listFilter) (listPage, error) {
	cur := start
	var matched []listedSession
	pages := 0
	truncated := false
	for pages < listACPPageCap && len(matched) < filter.limit {
		params := map[string]any{}
		if cwd != "" {
			params["cwd"] = cwd
		}
		if cur.ACP != "" {
			params["cursor"] = cur.ACP
		}
		var res struct {
			Sessions   []json.RawMessage `json:"sessions"`
			NextCursor string            `json:"nextCursor"`
		}
		if err := s.acpCall(ctx, "session/list", params, &res); err != nil {
			return listPage{}, err
		}
		pages++
		sent := cur.ACP
		if cur.At > len(res.Sessions) {
			cur.At = len(res.Sessions)
		}
		rows := res.Sessions
		if s.cfg.Profile != "zero" {
			rows = stripZeroMeta(rows)
		}
		filled := false
		for i := cur.At; i < len(rows); i++ {
			item, ok := s.decorateSession(rows[i], filter)
			if ok {
				matched = append(matched, item)
			}
			if len(matched) == filter.limit {
				cur = listCursor{ACP: sent, At: i + 1}
				if cur.At >= len(rows) {
					cur = listCursor{ACP: res.NextCursor}
				}
				filled = true
				break
			}
		}
		if filled {
			break
		}
		if res.NextCursor == "" || res.NextCursor == sent {
			cur = listCursor{}
			break
		}
		cur = listCursor{ACP: res.NextCursor}
		if pages == listACPPageCap {
			truncated = true
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].pinned && !matched[j].pinned
	})
	out := make([]json.RawMessage, len(matched))
	for i, item := range matched {
		out[i] = item.raw
	}
	next := ""
	if cur.ACP != "" || cur.At != 0 {
		next = encodeListCursor(cur)
	}
	if next == "" {
		truncated = false
	}
	return listPage{sessions: out, next: next, truncated: truncated}, nil
}

type listedSession struct {
	raw    json.RawMessage
	pinned bool
}

func (s *Server) decorateSession(raw json.RawMessage, filter listFilter) (listedSession, bool) {
	var view struct {
		Title string `json:"title"`
		Cwd   string `json:"cwd"`
		ID    string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		return listedSession{}, false
	}
	var entry sessionmeta.Entry
	if s.meta != nil {
		if got, ok := s.meta.Get(view.ID); ok {
			entry = got
		}
	}
	switch filter.archived {
	case "exclude":
		if entry.Archived {
			return listedSession{}, false
		}
	case "only":
		if !entry.Archived {
			return listedSession{}, false
		}
	}
	if !matchesQuery(filter.query, entry.Alias, view.Title, view.Cwd) {
		return listedSession{}, false
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return listedSession{}, false
	}
	obj["alias"] = entry.Alias
	obj["archived"] = entry.Archived
	obj["pinned"] = entry.Pinned
	b, err := json.Marshal(obj)
	if err != nil {
		return listedSession{}, false
	}
	return listedSession{raw: b, pinned: entry.Pinned}, true
}

func matchesQuery(q, alias, title, cwd string) bool {
	if q == "" {
		return true
	}
	needle := strings.ToLower(q)
	base := strings.ToLower(filepath.Base(cwd))
	return strings.Contains(strings.ToLower(alias), needle) ||
		strings.Contains(strings.ToLower(title), needle) ||
		strings.Contains(base, needle)
}
