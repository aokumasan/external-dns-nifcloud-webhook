package main

import (
	"log"

	"github.com/aokumasan/external-dns-nifcloud-webhook/cmd/webhook/app"
)

func main() {
	cmd := app.NewWebhookCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
