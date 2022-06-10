package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	cobbledayUrl, ok := os.LookupEnv("COBBLEDAY_URL")
	if !ok {
		cobbledayUrl = "http://localhost:8080"
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			var randMsg = make([]byte, rand.Intn(150)+50)
			_, err := rand.Read(randMsg)
			if err != nil {
				log.Printf("Failed to generate random message: %v", err)
				continue
			}

			err = sendCobbledayRequest(ctx, cobbledayUrl, string(randMsg))
			if err != nil {
				log.Printf("Failed to send message: %v", err)
				continue
			}

			time.Sleep(time.Duration(rand.Int31()))
		}
	}()

	<-sig
}

type Message struct {
	From    string `json:"from"`
	Message string `json:"message"`
}

func sendCobbledayRequest(ctx context.Context, url string, msg string) error {
	reqBody, err := json.Marshal(Message{
		From:    "paralipteron",
		Message: msg,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	return nil
}
