package app

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aokumasan/external-dns-nifcloud-webhook/internal/cloud"
	"github.com/spf13/cobra"
	"sigs.k8s.io/external-dns/provider/webhook"
)

const (
	defaultPort  = 8888
	readTimeout  = 30 * time.Second
	writeTimeout = 30 * time.Second
)

func NewWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "external-dns-nifcloud-webhook",
		Short: "provider webhook for the NIFCLOUD DNS service",
		Run: func(_ *cobra.Command, _ []string) {
			accessKey := os.Getenv("NIFCLOUD_ACCESS_KEY_ID")
			if accessKey == "" {
				log.Fatal("environment variable 'NIFCLOUD_ACCESS_KEY_ID' is required")
			}

			secretKey := os.Getenv("NIFCLOUD_SECRET_ACCESS_KEY")
			if secretKey == "" {
				log.Fatal("environment variable 'NIFCLOUD_SECRET_ACCESS_KEY' is required")
			}

			provider, err := cloud.NewNifcloudProvider(accessKey, secretKey)
			if err != nil {
				log.Fatalf("failed to create NIFCLOUD provider: %s", err)
			}

			port := defaultPort
			envPort := os.Getenv("PORT")
			if envPort != "" {
				port, err = strconv.Atoi(envPort)
				if err != nil {
					log.Fatalf("'PORT' is invalid: %s", err)
				}
			}

			addr := fmt.Sprintf(":%d", port)
			log.Printf("starting webhook server on %s", addr)
			webhook.StartHTTPApi(provider, nil, readTimeout, writeTimeout, addr)
		},
	}

	return cmd
}
