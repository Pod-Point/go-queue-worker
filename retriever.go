package formigo

import (
	"context"
	"errors"
	"fmt"

	"github.com/Pod-Point/go-queue-worker/internal/client"
	"github.com/Pod-Point/go-queue-worker/internal/messages"
)

// retriever will get messages from SQS until the given context gets canceled.
// Any error will be sent to the controller.
func retriever(ctx context.Context, receiver client.MessageReceiver, ctrl *controller, messageCh chan<- messages.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgs, err := receiver.ReceiveMessages(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
					// The worker's context was canceled. We can exit.
					return
				}

				// Report the error to the controller and continue.
				ctrl.reportError(fmt.Errorf("unable to receive message: %w", err))
				continue
			}

			// All the messages retrieved must be processed.
			// This means that the retriever won't listen for context cancellation
			// at this stage.
			func() {
				for _, msg := range msgs {
					select {
					case <-msg.Ctx.Done():
						// If consumers don't pick up the messages within the messages' timeout we raise
						// an error.
						// This could be due to one or more of the following reasons:
						// - message timeout too small.
						// - consumer too slow. Increasing the number of consumers may help, especially if
						//   the handler performs many I/O operations.
						//
						// Note that we won't process all messages retrieved by the API calls. This is because
						// the visibility timeout is the same for all the messages returned by the call.
						ctrl.reportError(errors.New("message didn't get picked up by any consumer within its timeout"))

						return // Avoid publishing all the messages downstream
					case messageCh <- msg:
						// Message pushed to the channel
					}
				}
			}()
		}
	}
}
