package main

import (
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/streadway/amqp"
)

func main() {
	amqpUrl, ok := os.LookupEnv("AMQP_URL")
	if !ok {
		amqpUrl = "amqp://guest:guest@localhost:5672/"
	}

	queue, err := amqp.Dial(amqpUrl)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer queue.Close()

	a := NewAlderstool(queue)
	w := NewWrong(queue)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	go func() {
		for {
			var msg = make([]byte, rand.Intn(328))
			rand.Read(msg)

			a.Submit(Message{From: "squawbar", Message: string(msg)})

			time.Sleep(time.Duration(rand.Int31() / 2))
		}
	}()

	go func() {
		for {
			var msg = make([]byte, rand.Intn(128))
			rand.Read(msg)

			w.Submit(Message{From: "squawbar", Message: string(msg)})

			time.Sleep(time.Duration((rand.Intn(80))+10) * time.Second)
		}
	}()

	<-sig
}
