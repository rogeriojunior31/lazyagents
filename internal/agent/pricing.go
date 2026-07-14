package agent

import "strings"

// pricePerMTok é o preço em USD por milhão de tokens.
type pricePerMTok struct {
	Input, Output, CacheRead, CacheWrite float64
}

// pricingTable é embutida e best-effort (sem chamada de rede — preços mudam,
// mas servem de estimativa). Chaves são prefixos do nome do modelo: o sufixo
// de data varia por release e o prefixo é estável o bastante para casar.
var pricingTable = map[string]pricePerMTok{
	"claude-opus-4":     {Input: 15, Output: 75, CacheRead: 1.5, CacheWrite: 18.75},
	"claude-3-opus":     {Input: 15, Output: 75, CacheRead: 1.5, CacheWrite: 18.75},
	"claude-sonnet-4":   {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"claude-3-5-sonnet": {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"claude-3-7-sonnet": {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"claude-haiku-4":    {Input: 1, Output: 5, CacheRead: 0.1, CacheWrite: 1.25},
	"claude-3-5-haiku":  {Input: 0.8, Output: 4, CacheRead: 0.08, CacheWrite: 1},
}

// EstimateCost calcula o custo em USD a partir do uso, se o modelo é
// reconhecido pela tabela embutida (por prefixo). ok=false = modelo
// desconhecido ou sem uso — o chamador mostra só os tokens.
func EstimateCost(u Usage) (cost float64, ok bool) {
	if u.Model == "" {
		return 0, false
	}
	for prefix, p := range pricingTable {
		if strings.HasPrefix(u.Model, prefix) {
			cost = float64(u.Input)/1e6*p.Input +
				float64(u.Output)/1e6*p.Output +
				float64(u.CacheRead)/1e6*p.CacheRead +
				float64(u.CacheWrite)/1e6*p.CacheWrite
			return cost, true
		}
	}
	return 0, false
}
