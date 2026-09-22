package agent

// Usage é o consumo de tokens agregado de uma sessão.
type Usage struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
	Model      string // último modelo visto na sessão (best-effort)
}

// UsageReader é implementado opcionalmente pelos adapters que sabem somar o
// uso de tokens de uma sessão. Fica fora da interface Adapter de propósito —
// nem todo agente registra usage no transcript — quem consome faz type
// assertion.
type UsageReader interface {
	// SessionUsage soma o usage da sessão. ok=false quando o agente não
	// registra usage ou o transcript não tem a informação.
	SessionUsage(s Session) (Usage, bool)
}
