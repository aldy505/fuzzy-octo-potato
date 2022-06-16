package main

import (
	"encoding/json"
	"log"

	"github.com/streadway/amqp"
)

type Message struct {
	From    string `json:"from"`
	Message string `json:"message"`
}

type Alderstool struct {
	msgs  chan Message
	queue *amqp.Connection
}

func NewAlderstool(queue *amqp.Connection) *Alderstool {
	msgsChan := make(chan Message)

	a := &Alderstool{queue: queue, msgs: msgsChan}
	go func() {
		err := a.publisher()
		if err != nil {
			log.Fatalf("Failed to publish messages: %v", err)
		}
	}()

	return a
}

func (a *Alderstool) publisher() error {
	ch, err := a.queue.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"alderstool", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return err
	}

	for msg := range a.msgs {
		encodedMsg, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Failed to encode message: %s", err)
			continue
		}

		err = ch.Publish(
			"",     // exchange
			q.Name, // routing key
			false,  // mandatory
			false,
			amqp.Publishing{
				DeliveryMode: amqp.Persistent,
				ContentType:  "application/json",
				Body:         encodedMsg,
			},
		)
		if err != nil {
			log.Printf("failed to publish msg: %v", err)
			continue
		}
	}

	return nil
}

func (a *Alderstool) Submit(msg Message) {
	a.msgs <- msg
}
