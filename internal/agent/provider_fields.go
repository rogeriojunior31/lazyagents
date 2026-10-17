package agent

// ProviderFieldReader projects configured public fields using the adapter's
// protocol defaults and credential precedence. Callers pass redacted profiles.
// Omitted optional fields do not constrain recognition; apply/clear handles them.
type ProviderFieldReader interface {
	ProviderFields(ProviderProfile) map[string]string
}

func providerFields(p ProviderProfile) map[string]string {
	p = p.Redacted()
	credential := "none from profile"
	if p.HasToken {
		credential = "token present (value not compared)"
	} else if p.EnvKey != "" {
		credential = "env $" + p.EnvKey
	}
	return map[string]string{"endpoint": p.BaseURL, "model": p.Model, "protocol": p.WireAPI, "credential": credential}
}

func (c *Claude) ProviderFields(p ProviderProfile) map[string]string {
	fields := providerFields(p)
	fields["protocol"] = "anthropic"
	if p.BaseURL == "" {
		delete(fields, "endpoint")
	}
	if p.Model == "" {
		delete(fields, "model")
	}
	if p.Token == "" && !p.HasToken {
		delete(fields, "credential")
	}
	return fields
}

func (c *Codex) ProviderFields(p ProviderProfile) map[string]string {
	fields := providerFields(p)
	if p.WireAPI == "" {
		fields["protocol"] = "responses"
	}
	fields["credential"] = "none from profile"
	if p.EnvKey != "" {
		fields["credential"] = "env $" + p.EnvKey
	} else if p.Token != "" || p.HasToken {
		fields["credential"] = "unsupported saved token; env reference required"
	}
	if p.Model == "" {
		delete(fields, "model")
	}
	return fields
}

func (p *Pi) ProviderFields(pr ProviderProfile) map[string]string {
	fields := providerFields(pr)
	switch piWireAPIs[pr.WireAPI] {
	case "openai-completions":
		fields["protocol"] = "chat"
	case "openai-responses":
		fields["protocol"] = "responses"
	case "anthropic-messages":
		fields["protocol"] = "anthropic"
	}
	return fields
}

func (c *Crush) ProviderFields(p ProviderProfile) map[string]string {
	fields := providerFields(p)
	if p.WireAPI == "" {
		fields["protocol"] = "chat"
	}
	return fields
}
