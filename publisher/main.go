package main

import (
	"context"
	"flag"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
)

func main() {
	project := flag.String("project", "", "gcp project")
	topic := flag.String("topic", "", "pub/sub topic")
	n := flag.Int("n", 0, "number of messages to publish")
	flag.Parse()

	if *topic == "" || *project == "" || *n < 1 {
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	c, err := pubsub.NewClient(ctx, *project)
	if err != nil {
		log.Fatal(err)
	}

	t := c.Topic(*topic)

	for i := 0; i < *n; i++ {
		// TODO: error handling
		t.Publish(ctx, &pubsub.Message{
			Data: []byte("foo"),
		})
	}

	t.Stop()
	if err := c.Close(); err != nil {
		log.Fatal("could not close client: ", err)
	}

	log.Printf("published %d messages", *n)
}
