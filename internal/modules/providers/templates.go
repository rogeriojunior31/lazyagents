package providers

import "github.com/rogeriojunior31/lazyagents/internal/agent"

// Templates are starting values, not a new profile format or a runtime catalog.
type connectionTemplate struct {
	name, summary, protocol, required, credential string
	agents                                        []string
	source, reviewed                              string
	profile                                       agent.ProviderProfile
	defaults                                      bool
}

var connectionTemplates = []connectionTemplate{
	{
		name:       "Custom connection",
		summary:    "Enter your own endpoint and protocol; every field stays editable.",
		agents:     []string{"claude-code", "codex", "gemini-cli", "opencode", "pi", "crush"},
		protocol:   "Match the endpoint to each destination agent; no protocol conversion is provided.",
		required:   "Name and at least one change; endpoint/model/credentials depend on the selected agents.",
		credential: "Saved token or an environment reference, according to the agent. Native login stays in the agent.",
		source:     "https://rogeriojunior31.github.io/docs/lazyagents/guide/providers/", reviewed: "2026-10-17",
	},
	{
		name: "Agent defaults / previous settings", defaults: true,
		summary:    "Remove managed provider overrides from agents you explicitly select; no profile is saved.",
		agents:     []string{"claude-code", "codex", "gemini-cli", "opencode", "pi", "crush"},
		protocol:   "The agent chooses its own connection after managed overrides are cleared.",
		required:   "Select installed agents, then review and confirm the clear operation.",
		credential: "Sign in through the native agent. Previous or external settings may still select another provider.",
		source:     "https://rogeriojunior31.github.io/docs/lazyagents/guide/providers/", reviewed: "2026-10-17",
	},
	{
		name:       "Gemini GenerateContent",
		summary:    "Direct Gemini API connection; enter an exact tool-capable model ID.",
		agents:     []string{"gemini-cli"},
		protocol:   "GenerateContent (wire api: gemini). Use the base URL without /models or an operation path. OpenAI-compatible catalogs are unavailable; enter the model manually.",
		required:   "Profile name, HTTPS base URL (HTTP only on localhost), model and a saved API key or GEMINI_API_KEY.",
		credential: "Only the fixed GEMINI_API_KEY variable is supported. Saved keys go into a private managed dotenv block. Shell variables and nearer .env files may override the home connection. Restart Gemini in a trusted workspace; manage trust in Gemini itself.",
		source:     "https://geminicli.com/docs/reference/configuration/", reviewed: "2026-10-18",
		profile: agent.ProviderProfile{Name: "gemini-api", BaseURL: "https://generativelanguage.googleapis.com", WireAPI: "gemini", EnvKey: "GEMINI_API_KEY"},
	},
	{
		name:    "OpenAI Responses",
		summary: "Direct OpenAI API connection using Responses; choose a model available to your API account.",
		agents:  []string{"codex", "pi"}, protocol: "Responses (wire api: responses). Keep the /v1 base path; do not append /responses.",
		required:   "Profile name, model, and OPENAI_API_KEY exported in the agent's environment.",
		credential: "Only the variable name is filled in. Codex never receives a saved token. Choose agent defaults for native account sign-in.",
		source:     "https://developers.openai.com/api/reference/resources/responses/methods/create", reviewed: "2026-10-17",
		profile: agent.ProviderProfile{Name: "openai-responses", BaseURL: "https://api.openai.com/v1", EnvKey: "OPENAI_API_KEY", WireAPI: "responses"},
	},
	{
		name:    "OpenAI Chat Completions",
		summary: "Direct OpenAI API connection using Chat Completions; choose a model available to your API account.",
		agents:  []string{"opencode", "pi", "crush"}, protocol: "Chat Completions (wire api: chat). Keep the /v1 base path; do not append /chat/completions. Codex requires Responses.",
		required:   "Profile name, model, and OPENAI_API_KEY exported in the agent's environment.",
		credential: "The variable name is filled in; a saved token is optional and takes precedence for Pi/Crush. Authentication is not checked here.",
		source:     "https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create", reviewed: "2026-10-17",
		profile: agent.ProviderProfile{Name: "openai-chat", BaseURL: "https://api.openai.com/v1", EnvKey: "OPENAI_API_KEY", WireAPI: "chat"},
	},
	{
		name:    "Anthropic gateway for Claude Code",
		summary: "Connect directly to an existing Anthropic Messages gateway that accepts bearer tokens.",
		agents:  []string{"claude-code"}, protocol: "Anthropic Messages. Use the gateway's base URL; Claude appends /v1/messages. lazyagents does not run or convert a gateway.",
		required:   "Profile name, gateway base URL, gateway model and saved bearer token.",
		credential: "Claude writes the saved token as ANTHROPIC_AUTH_TOKEN (Authorization: Bearer). env var cannot supply it. For x-api-key or native sign-in, configure the agent itself.",
		source:     "https://code.claude.com/docs/en/llm-gateway-connect", reviewed: "2026-10-17",
		profile: agent.ProviderProfile{Name: "anthropic-gateway", WireAPI: "anthropic"},
	},
	{
		name:    "Ollama local · Chat Completions",
		summary: "Connect to a local Ollama server; enter the exact name of a downloaded tool-capable model.",
		agents:  []string{"opencode", "pi", "crush"}, protocol: "OpenAI-compatible Chat Completions (wire api: chat). The local server uses port 11434 and the /v1 base path.",
		required:   "Profile name, running local Ollama server and downloaded model name. Keep the endpoint local for this keyless setup.",
		credential: "Local Ollama does not require authentication. Leave token and env var empty; Pi supplies its keyless placeholder. Cloud or authenticated servers need custom credentials.",
		source:     "https://docs.ollama.com/api/openai-compatibility", reviewed: "2026-10-17",
		profile: agent.ProviderProfile{Name: "ollama-local", BaseURL: "http://localhost:11434/v1", WireAPI: "chat"},
	},
}
