package bus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

var ErrSubjectRequired = errors.New("subject is required")

type Publisher interface {
	PublishJSON(ctx context.Context, subject string, payload any, headers map[string]string) error
	Close() error
	Enabled() bool
}

type noopPublisher struct{}

func (p noopPublisher) PublishJSON(_ context.Context, _ string, _ any, _ map[string]string) error {
	return nil
}

func (p noopPublisher) Close() error {
	return nil
}

func (p noopPublisher) Enabled() bool {
	return false
}

type natsPublisher struct {
	connection *nats.Conn
	prefix     string
}

func NewPublisher(enabled bool, serverURL, subjectPrefix string) (Publisher, error) {
	if !enabled {
		return noopPublisher{}, nil
	}
	nextServerURL := strings.TrimSpace(serverURL)
	if nextServerURL == "" {
		return nil, errors.New("nats server url is required")
	}
	conn, err := nats.Connect(
		nextServerURL,
		nats.Name("mistypass-internal-bus"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return &natsPublisher{
		connection: conn,
		prefix:     normalizeSubjectPrefix(subjectPrefix),
	}, nil
}

func (p *natsPublisher) PublishJSON(
	ctx context.Context,
	subject string,
	payload any,
	headers map[string]string,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	nextSubject := strings.TrimSpace(subject)
	if nextSubject == "" {
		return ErrSubjectRequired
	}
	if p == nil || p.connection == nil {
		return errors.New("nats publisher connection is not initialized")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	message := nats.NewMsg(qualifiedSubject(p.prefix, nextSubject))
	message.Data = body
	for key, value := range headers {
		nextKey := strings.TrimSpace(key)
		if nextKey == "" {
			continue
		}
		message.Header.Set(nextKey, strings.TrimSpace(value))
	}
	if err := p.connection.PublishMsg(message); err != nil {
		return err
	}
	return p.connection.FlushWithContext(ctx)
}

func (p *natsPublisher) Close() error {
	if p.connection == nil {
		return nil
	}
	p.connection.Flush()
	p.connection.Close()
	return nil
}

func (p *natsPublisher) Enabled() bool {
	return true
}

func normalizeSubjectPrefix(prefix string) string {
	next := strings.Trim(strings.TrimSpace(prefix), ".")
	if next == "" {
		return "mistypass"
	}
	return next
}

func qualifiedSubject(prefix, subject string) string {
	nextPrefix := normalizeSubjectPrefix(prefix)
	nextSubject := strings.Trim(strings.TrimSpace(subject), ".")
	if nextSubject == "" {
		return nextPrefix
	}
	return nextPrefix + "." + nextSubject
}
