package main

import (
	"fmt"
	"log"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	user:=os.Getenv("USER_RM")
	password:=os.Getenv("PASSWORD_RM")
	svc:=os.Getenv("SVC_RM")
	connRabbitMQ, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s/",user,password,svc))
	if err != nil {
		panic(err)
	}

	// Open a new channel.
	channel, err := connRabbitMQ.Channel()
	if err != nil {
		log.Println(err)
	}
	defer channel.Close()

	// Start receiving queued messages.
	messages, err := channel.Consume(
		"TestQueue",
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println(err)
	}

	// Welcome message.
	log.Println("Successfully connected to RabbitMQ instance")
	log.Println("[*] - Waiting for messages")

	// Open a channel to receive messages.
	forever := make(chan bool)

	go func() {
		for message := range messages {
			// For example, just show received message in console.
			log.Printf("Received message: %s\n", message.Body)
		}
	}()

	// Close the channel.
	<-forever
}