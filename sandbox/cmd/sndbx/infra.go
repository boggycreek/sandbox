// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/doctor"
	"github.com/boggycreek/sandbox/pkg/runtime"
)

func handleInfra(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx infra <up|down|list|doctor>")
		return 1
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, `Usage: sndbx infra <command>

Commands:
  up      Start shared Valkey, Gitea, and PostgreSQL services via Podman Compose
  down    Stop shared infrastructure services
  list    Show status of running infrastructure containers
  doctor  Diagnose shared infrastructure storage, containers, and services, and auto-heal defects`)
		return 0

	case "up":
		fmt.Fprintln(stdout, "Starting shared infrastructure (Valkey, Gitea & PostgreSQL)...")
		adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
		humanPass := os.Getenv("HUMAN_BACKPLANE_PASSWORD")
		humanName := os.Getenv("HUMAN_NAME")
		if err := runtime.StartInfraStack(ctx, paths, adminPass, humanPass, humanName); err != nil {
			fmt.Fprintf(stderr, "sndbx infra error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Shared infrastructure is online.")
		return 0

	case "down":
		fmt.Fprintln(stdout, "Stopping shared infrastructure...")
		if err := runtime.StopInfraStack(ctx); err != nil {
			fmt.Fprintf(stderr, "sndbx infra error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Shared infrastructure stopped.")
		return 0

	case "doctor":
		report, err := doctor.DiagnoseAndHealInfra(ctx, paths)
		if err != nil && report == nil {
			fmt.Fprintf(stderr, "sndbx infra doctor error: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, doctor.FormatDoctorReport(report))
		if report.UnrepairableCount > 0 {
			return 1
		}
		return 0

	case "list":
		fmt.Fprintln(stdout, "Shared infrastructure status:")
		list, err := runtime.InspectInfraStack(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "sndbx infra error: %v\n", err)
			return 1
		}
		fmtFmt := "%-28s %s\n"
		fmt.Fprintf(stdout, fmtFmt, "SERVICE", "STATUS")
		for _, item := range list {
			displayName := item.ID
			if len(item.Names) > 0 && item.Names[0] != "" {
				displayName = item.Names[0]
			}
			fmt.Fprintf(stdout, fmtFmt, displayName, item.State)
		}
		return 0

	default:
		fmt.Fprintf(stderr, "sndbx infra: unknown command %q\n", sub)
		return 1
	}
}
