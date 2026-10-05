package formigo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type TestClient struct {
	//
}

func (t TestClient) Fetch(ctx context.Context) ([]Message, error) {
	return nil, nil
}

func (t TestClient) Delete(ctx context.Context, message Message) error {
	return nil
}

var _ Client = (*TestClient)(nil)

// testContext is a safety measure for testing Run in case there is never an exit condition.
func testContext() (context.Context, context.CancelFunc) {
	return context.WithDeadline(context.Background(), time.Now().Add(time.Second*4))
}

func TestNewDefaultManager_VerifiesDefaults(t *testing.T) {
	manager := NewDefaultManager(TestClient{})

	assert.Nil(t, manager.consumer)
	assert.Nil(t, manager.reporter)
	assert.Nil(t, manager.logger)

	assert.Equal(t, 2, manager.fetchConcurrency)
	assert.Equal(t, 20, manager.workerConcurrency)
	assert.Equal(t, 0, manager.fetchDelay)
	assert.Equal(t, time.Second*30, manager.deadline)
}

func TestManager_Run_FailsWithoutConsumer(t *testing.T) {
	manager := NewDefaultManager(TestClient{})
	ctx, cancel := testContext()
	defer cancel()

	assert.ErrorIs(t, manager.Run(ctx), ErrConsumerMissing)
}
