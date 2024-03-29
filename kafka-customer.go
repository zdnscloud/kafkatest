package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
)

func main() {
	var addr string
	flag.StringVar(&addr, "addr", "10.42.1.40:9092", "kafka ep")
	flag.Parse()
	var Address = []string{addr}
	topic := []string{"test"}
	var wg = &sync.WaitGroup{}
	wg.Add(2)
	go clusterConsumer(wg, Address, topic, "group-1")
	go clusterConsumer(wg, Address, topic, "group-2")

	wg.Wait()
}

func clusterConsumer(wg *sync.WaitGroup, brokers, topics []string, groupId string) {
	defer wg.Done()
	config := cluster.NewConfig()
	config.Consumer.Return.Errors = true
	config.Group.Return.Notifications = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	// init consumer
	consumer, err := cluster.NewConsumer(brokers, groupId, topics, config)
	if err != nil {
		log.Printf("%s: sarama-cluster.NewConsumer err, message=%s \n", groupId, err)
		return
	}
	defer consumer.Close()

	// trap SIGINT to trigger a shutdown
	signals := make(chan os.Signal)
	signal.Notify(signals,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		os.Interrupt,
		os.Kill,
	)

	// consume errors
	go func() {
		for err := range consumer.Errors() {
			log.Printf("groupId=%s, Error= %s\n", groupId, err.Error())
		}
	}()

	// consume notifications
	go func() {
		for ntf := range consumer.Notifications() {
			log.Printf("groupId=%s, Rebalanced Info= %+v \n", groupId, ntf)
		}
	}()

	// consume messages, watch signals
	var successes int
Loop:
	for {
		select {
		case msg, ok := <-consumer.Messages():
			if ok {
				fmt.Fprintf(os.Stdout, "groupId=%s, topic=%s, partion=%d, offset=%d, key=%s, value=%s\n", groupId, msg.Topic, msg.Partition, msg.Offset, msg.Key, msg.Value)
				consumer.MarkOffset(msg, "") // mark message as processed
				successes++
			}
		case <-signals:
			break Loop
		}
	}
	fmt.Fprintf(os.Stdout, "%s consume %d messages \n", groupId, successes)
}
