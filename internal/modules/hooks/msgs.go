package hooks

// loadedMsg traz a biblioteca e o estado de cada agente.
type loadedMsg struct {
	lib      []Hook
	problems []string
	statuses []Status
}

// doneMsg é o resultado de uma escrita (enable, disable, delete).
type doneMsg struct {
	text string
	err  bool
}

// libraryMsg é a leitura leve do boot: só a biblioteca, para o contador.
type libraryMsg struct{ lib []Hook }
