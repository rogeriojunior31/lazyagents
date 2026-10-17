package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

func providerConnection(operation string, args []string, c cli.Context, svc *Service) int {
	fs := cli.Flags("provider "+operation, c.Err)
	jsonOut := fs.Bool("json", false, "JSON output")
	name, ok := firstArg(fs, args)
	if !ok || fs.NArg() != 0 {
		fmt.Fprintf(c.Err, "usage: lazyagents provider %s <profile> [--json]\n", operation)
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	result := svc.CheckSaved(ctx, name, operation == "models")
	if *jsonOut {
		enc := json.NewEncoder(c.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintln(c.Err, "writing connection result failed")
			return 1
		}
	} else {
		printConnection(c.Out, result)
		for _, id := range result.Models {
			fmt.Fprintln(c.Out, id)
		}
	}
	if result.Error != "" {
		return 1
	}
	return 0
}

func printConnection(out io.Writer, r ConnectionResult) {
	if r.Reachable {
		fmt.Fprintf(out, "Endpoint responded: HTTP %d\n", r.HTTPStatus)
	} else {
		fmt.Fprintln(out, "Endpoint response: not observed")
	}
	switch r.Credential {
	case "configured":
		fmt.Fprintln(out, "Credentials: configured for the request; server acceptance was not observed")
	case "sent":
		fmt.Fprintln(out, "Credentials: sent to the configured catalog; authentication is not independently proven")
	case "rejected":
		fmt.Fprintln(out, "Credentials: HTTP 401; authentication not accepted")
	case "forbidden":
		fmt.Fprintln(out, "Credentials: HTTP 403; access forbidden")
	default:
		fmt.Fprintln(out, "Credentials: not sent; authentication not tested")
	}
	if r.Catalog {
		fmt.Fprintf(out, "Catalog: %d models returned\nSelected model: %s\n", len(r.Models), r.ModelStatus)
	}
	if r.Error != "" {
		fmt.Fprintln(out, r.Error)
	}
	fmt.Fprintln(out, "No inference sent. No profile or agent settings changed.")
}
