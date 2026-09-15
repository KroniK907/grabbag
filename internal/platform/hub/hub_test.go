package hub_test

import (
	"bufio"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KroniK907/hackbox/internal/platform/hub"
)

func TestPublishSendsNamedEvent(t *testing.T) {
	t.Parallel()

	events := hub.New()
	server := httptest.NewServer(events)
	t.Cleanup(server.Close)

	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })

	reader := bufio.NewReader(response.Body)
	readEvent(t, reader, ": connected")

	events.Publish("roster")

	event := readEvent(t, reader, "event: roster")
	if !strings.Contains(event, "data: update") {
		t.Fatalf("event = %q, want update data", event)
	}
}

func readEvent(t *testing.T, reader *bufio.Reader, want string) string {
	t.Helper()

	var event strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		event.WriteString(line)
		if line == "\n" {
			break
		}
	}
	if !strings.Contains(event.String(), want) {
		t.Fatalf("event = %q, want %q", event.String(), want)
	}
	return event.String()
}
