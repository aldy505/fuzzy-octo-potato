package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
	ginprometheus "github.com/zsais/go-gin-prometheus"
)

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

	queue, err := amqp.Dial(amqpUrl)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer queue.Close()

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	err = migrate(db)
	if err != nil {
		log.Printf("Failed to migrate database: %v", err)
		return
	}

	a := NewAlderstool(queue)

	r := gin.New()

	p := ginprometheus.NewPrometheus("gin")
	p.Use(r)

	r.GET("/healthz", func(c *gin.Context) {
		a.Submit(Message{From: "cobbleday", Message: "healthz endpoint was called"})
		c.JSON(200, gin.H{
			"message": "ok",
		})
	})

	r.POST("/", func(c *gin.Context) {
		var msg Message
		err := c.BindJSON(&msg)
		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		hash := sha256.Sum256([]byte(msg.Message))
		encodedHash := hex.EncodeToString(hash[:])

		conn, err := db.Conn(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}
		defer conn.Close()

		_, err = conn.ExecContext(c.Request.Context(), "INSERT INTO cobbleday (hashed, message, created_at) VALUES ($1, $2, NOW())", encodedHash, msg.Message)
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}

		a.Submit(Message{From: "cobbleday", Message: "a message was inserted to cobbleday table"})

		c.JSON(200, gin.H{
			"message": "ok",
		})
	})

	r.Run(":" + port)
}
