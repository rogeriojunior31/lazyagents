package agent

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

// V2 stores one mutable row per assistant response; query metadata, never content/credentials.
func (o *OpenCode) UsageEvents(s Session) ([]UsageEvent, error) {
	if _, err := os.Stat(o.dbPath()); os.IsNotExist(err) {
		return nil, nil
	}
	sqlite, err := o.Look("sqlite3")
	if err != nil {
		return nil, fmt.Errorf("OpenCode usage requires sqlite3")
	}
	v2, err := o.hasV2Sessions(sqlite)
	if err != nil {
		return nil, fmt.Errorf("OpenCode usage schema is unavailable")
	}
	if !v2 {
		return nil, nil
	} // Older formats have no independently verified usage reader yet.
	id := strings.ReplaceAll(s.ID, "'", "''")
	var rows []struct {
		At        int64    `json:"at"`
		Completed *int64   `json:"completed"`
		Model     string   `json:"model"`
		Provider  string   `json:"provider"`
		Tokens    string   `json:"tokens"`
		Cost      *float64 `json:"cost"`
	}
	q := fmt.Sprintf(`SELECT time_created AS at, json_extract(data,'$.time.completed') AS completed,
 json_extract(data,'$.model.id') AS model, json_extract(data,'$.model.providerID') AS provider,
 COALESCE(json_extract(data,'$.tokens'),'null') AS tokens, json_extract(data,'$.cost') AS cost
 FROM session_message WHERE session_id='%s' AND type='assistant' ORDER BY seq`, id)
	if err := o.queryJSON(sqlite, q, &rows); err != nil {
		return nil, fmt.Errorf("OpenCode usage metadata is unavailable")
	}
	var events []UsageEvent
	for _, row := range rows {
		if row.At <= 0 {
			return nil, fmt.Errorf("OpenCode usage contains a response without a valid timestamp")
		}
		u := Usage{Model: row.Model, Incomplete: true}
		var n struct {
			Input, Output, Reasoning *int
			Cache                    *struct{ Read, Write *int }
		}
		if json.Unmarshal([]byte(row.Tokens), &n) == nil && n.Input != nil && n.Output != nil && n.Reasoning != nil && n.Cache != nil && n.Cache.Read != nil && n.Cache.Write != nil && *n.Input >= 0 && *n.Output >= 0 && *n.Reasoning >= 0 && *n.Cache.Read >= 0 && *n.Cache.Write >= 0 {
			// OpenCode separates cache input and reasoning; output includes both generated categories here.
			u.Input, u.Output, u.CacheRead, u.CacheWrite = *n.Input, *n.Output+*n.Reasoning, *n.Cache.Read, *n.Cache.Write
			u.Incomplete = row.Completed == nil || *row.Completed <= 0
		}
		if row.Cost != nil && row.Completed != nil && *row.Completed > 0 && *row.Cost >= 0 && !math.IsInf(*row.Cost, 0) && !math.IsNaN(*row.Cost) {
			u.Cost, u.CostKnown = *row.Cost, true
		}
		events = append(events, UsageEvent{AgentID: o.ID(), Time: time.UnixMilli(row.At).UTC(), Model: row.Model, Provider: row.Provider, CWD: s.CWD, Usage: u, N: 1})
	}
	return events, nil
}
func (o *OpenCode) SessionUsage(s Session) (Usage, bool) {
	events, err := o.UsageEvents(s)
	if err != nil || len(events) == 0 {
		return Usage{}, false
	}
	return sumRecordedUsage(events), true
}
