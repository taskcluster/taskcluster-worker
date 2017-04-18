package fakequeue

import (
	"fmt"

	"github.com/streadway/amqp"
	"github.com/taskcluster/slugid-go/slugid"
)

type pulseListener struct {
	conn     *amqp.Connection
	username string
}

// NewPulseListener creates a listener implementation for production queue
// using pulse.
func NewPulseListener(username, password string) (Listener, error) {
	u := fmt.Sprintf("amqps://%s:%s@pulse.mozilla.org:5671", username, password)
	conn, err := amqp.Dial(u)
	if err != nil {
		return nil, err
	}
	return &pulseListener{
		conn:     conn,
		username: username,
	}, nil
}

var taskResolvedExchanges = []string{
	"exchange/taskcluster-queue/v1/task-completed",
	"exchange/taskcluster-queue/v1/task-failed",
	"exchange/taskcluster-queue/v1/task-exception",
}

func (l *pulseListener) WaitForTask(taskID string) <-chan error {
	done := make(chan error)

	channel, err := l.conn.Channel()
	if err != nil {
		go func() { done <- err }()
		return done
	}

	q, err := channel.QueueDeclare(
		fmt.Sprintf("queue/%s/%s", l.username, slugid.Nice()), false, true, true, false, nil,
	)
	if err != nil {
		go func() { done <- err }()
		return done
	}

	for _, ex := range taskResolvedExchanges {
		err = channel.QueueBind(q.Name, fmt.Sprintf("primary.%s.#", taskID), ex, false, nil)
		if err != nil {
			go func() { done <- err }()
			return done
		}
	}

	// require exclusive and autoAck because we don't care in this setting
	messages, err := channel.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		go func() { done <- err }()
		return done
	}

	errChan := make(chan *amqp.Error)
	channel.NotifyClose(errChan)
	go func() {
		select {
		case <-messages:
			done <- nil
		case e := <-errChan:
			done <- e
		}
		channel.Close()
		for range messages {
			// Ignore messages
		}
	}()
	return done
}
