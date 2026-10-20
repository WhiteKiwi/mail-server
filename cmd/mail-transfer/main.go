package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/whitekiwi/mail-server/internal/transfer"
)

func main() {
	input := flag.String("input", "", "protected PostgreSQL18 CSV snapshot")
	dsn := flag.String("database-url-file", "", "protected target URL; never pass a URL in argv")
	expected := flag.String("sha256", "", "source CSV SHA256")
	verify := flag.Bool("verify-only", false, "compare current target without writes")
	dryRun := flag.Bool("dry-run", false, "import and compare an empty target, then roll back")
	flag.Parse()
	if flag.NArg() != 0 || *input == "" || *dsn == "" {
		fmt.Fprintln(os.Stderr, "transfer input rejected")
		os.Exit(2)
	}
	data, err := transfer.Protected(*input, 1<<20)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	url, err := transfer.Protected(*dsn, 8192)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	count, err := transfer.Apply(ctx, strings.TrimSpace(string(url)), data, *expected, *verify, *dryRun)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"rows": count, "sha256": *expected, "verified": true, "verify_only": *verify, "dry_run": *dryRun})
}
