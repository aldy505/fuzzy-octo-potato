package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
)

type Message struct {
	From    string `json:"from"`
	Message string `json:"message"`
}

func main() {
	amqpUrl, ok := os.LookupEnv("AMQP_URL")
	if !ok {
		amqpUrl = "amqp://guest:guest@localhost:5672/"
	}

	dbUrl, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		dbUrl = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}

	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Minute * 1)
	db.SetConnMaxIdleTime(time.Second * 30)

	queue, err := amqp.Dial(amqpUrl)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %s", err)
	}
	defer queue.Close()

	err = migrate(db)
	if err != nil {
		log.Printf("Failed to migrate database: %s", err)
		return
	}

	ch, err := queue.Channel()
	if err != nil {
		log.Printf("Failed to open a channel: %s", err)
		return
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
		log.Printf("Failed to declare a queue: %s", err)
		return
	}

	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		log.Printf("Failed to set QoS: %s", err)
		return
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Printf("Failed to register a consumer: %s", err)
		return
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	go func() {
		for d := range msgs {
			var payload Message
			err := json.Unmarshal(d.Body, &payload)
			if err != nil {
				log.Printf("Failed to unmarshal message: %s", err)
				d.Nack(false, false)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()

			conn, err := db.Conn(ctx)
			if err != nil {
				log.Printf("Failed to connect to database: %s", err)
				d.Nack(false, false)
				continue
			}

			tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
			if err != nil {
				log.Printf("Failed to begin a transaction: %s", err)
				d.Nack(false, false)
				continue
			}

			_, err = tx.ExecContext(
				ctx,
				`INSERT INTO alderstool (from_user, message) VALUES ($1, $2)`,
				payload.From,
				payload.Message,
			)
			if err != nil {
				log.Printf("Failed to insert message: %s", err)
				d.Nack(false, false)
				continue
			}

			err = tx.Commit()
			if err != nil {
				log.Printf("Failed to commit transaction: %s", err)
				d.Nack(false, false)
				continue
			}

			err = conn.Close()
			if err != nil {
				log.Printf("Failed to close connection: %s", err)
				d.Nack(false, false)
				continue
			}

			d.Ack(false)
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		dbStat := "ok"
		queueStat := "ok"

		dbCtx, dbCancel := context.WithTimeout(r.Context(), time.Second*5)
		defer dbCancel()

		err := db.PingContext(dbCtx)
		if err != nil {
			dbStat = err.Error()
		}

		if queue.IsClosed() {
			queueStat = "closed"
		}

		if dbStat != "ok" || queueStat != "ok" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	httpSrv := &http.Server{
		Handler: mux,
		Addr:    ":" + port,
	}

	go func() {
		log.Printf("Listening on port %s", port)
		err := httpSrv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Failed to start HTTP server: %s", err)
		}
	}()

	<-sig

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second*5)
	defer shutdownCancel()

	err = httpSrv.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf("Failed to shutdown HTTP server: %s", err)
	}
}

func migrate(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`CREATE TABLE IF NOT EXISTS alderstool (
			id SERIAL PRIMARY KEY,
			from_user VARCHAR(255) NOT NULL,
			message VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
	)
	if err != nil {
		if e := tx.Rollback(); e != nil {
			return e
		}

		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
