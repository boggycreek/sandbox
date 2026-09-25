// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package lifecycle manages provisioning and deprovisioning of agent infrastructure.
package lifecycle

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/boggycreek/sandbox/pkg/gitea"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/pkg/sonar"
)

// DeprovisionValkeyUser removes the agent's Valkey ACL user, streams, and identity registration.
func DeprovisionValkeyUser(ctx context.Context, agentName string) error {
	bpCfg := libbp.LoadClientFromEnv()
	client, err := libbp.Dial(ctx, libbp.ClientConfig{
		Host:     bpCfg.Host,
		Port:     bpCfg.Port,
		Username: "admin",
		Password: os.Getenv("ADMIN_BACKPLANE_PASSWORD"),
	})
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	// Delete ACL user
	if _, execErr := client.Exec(ctx, "ACL", "DELUSER", agentName); execErr != nil {
		return execErr
	}

	// Clean up backplane keys & identity
	_, err = client.Exec(ctx, "DEL",
		fmt.Sprintf("identity:%s", agentName),
		fmt.Sprintf("%s:inbox", agentName),
		fmt.Sprintf("%s:out", agentName),
		fmt.Sprintf("%s:seq", agentName),
		fmt.Sprintf("%s:finger", agentName),
		fmt.Sprintf("%s:status", agentName),
	)
	return err
}

// DeprovisionGiteaUser removes the agent account and public keys from the local Gitea instance.
func DeprovisionGiteaUser(ctx context.Context, agentName string) error {
	adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	if adminPass == "" {
		adminPass = "admin_backplane_pass"
	}

	giteaURL := os.Getenv("GITEA_URL")
	if giteaURL == "" {
		giteaURL = "http://127.0.0.1:3000"
	}

	client := gitea.NewClient(gitea.ClientConfig{
		BaseURL:   giteaURL,
		AdminUser: "giteaadmin",
		AdminPass: adminPass,
		Timeout:   3 * time.Second,
	})

	return client.DeleteUser(ctx, agentName, true)
}

// DeprovisionSonarUser revokes the agent's analysis token and deactivates the agent account in SonarQube.
func DeprovisionSonarUser(ctx context.Context, agentName string) error {
	adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	if adminPass == "" {
		adminPass = "admin"
	}
	adminUser := os.Getenv("SONAR_ADMIN_USER")
	if adminUser == "" {
		adminUser = "admin"
	}

	sonarURL := os.Getenv("SONAR_HOST_URL")
	if sonarURL == "" {
		sonarURL = os.Getenv("SONARQUBE_URL")
	}
	if sonarURL == "" {
		sonarURL = "http://127.0.0.1:9000"
	}

	client := sonar.NewClient(sonar.ClientConfig{
		BaseURL:   sonarURL,
		AdminUser: adminUser,
		AdminPass: adminPass,
		Timeout:   2 * time.Second,
	})

	_ = client.RevokeUserToken(ctx, agentName, fmt.Sprintf("%s-agent-token", agentName))
	return client.DeactivateUser(ctx, agentName)
}
