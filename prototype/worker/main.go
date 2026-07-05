// Insucar async worker — consumes EventBridge->SQS work items (dispatch queue).
// Long-polls SQS, processes each message, deletes on success (failures fall to
// the DLQ after maxReceiveCount). Runs under the insucar-api IRSA (SQS perms).
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	queueURL := os.Getenv("DISPATCH_QUEUE_URL")
	if queueURL == "" {
		// SQS not provisioned yet (Terraform not applied). Idle instead of crashing so
		// the deployment stays healthy; real processing resumes once DISPATCH_QUEUE_URL is set.
		log.Printf(`{"stream":"system","event":"worker_idle","reason":"DISPATCH_QUEUE_URL not set — SQS not provisioned"}`)
		<-ctx.Done()
		log.Printf(`{"stream":"system","event":"worker_stop"}`)
		return
	}
	cfg, err := awscfg.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}
	client := sqs.NewFromConfig(cfg)

	log.Printf(`{"stream":"system","event":"worker_start","queue":%q}`, queueURL)
	for ctx.Err() == nil {
		out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
			VisibilityTimeout:   60,
		})
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Printf(`{"stream":"error","event":"sqs_receive_failed","err":%q}`, err.Error())
			time.Sleep(2 * time.Second)
			continue
		}
		for _, m := range out.Messages {
			if process(m) {
				if _, derr := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl: aws.String(queueURL), ReceiptHandle: m.ReceiptHandle,
				}); derr != nil {
					log.Printf(`{"stream":"error","event":"sqs_delete_failed","err":%q}`, derr.Error())
				}
			}
		}
	}
	log.Printf(`{"stream":"system","event":"worker_stop"}`)
}

// process handles one EventBridge->SQS envelope. Returns true when the message
// was handled (and can be deleted); false leaves it for retry/DLQ.
func process(m sqstypes.Message) bool {
	var env struct {
		DetailType string          `json:"detail-type"`
		Source     string          `json:"source"`
		Detail     json.RawMessage `json:"detail"`
	}
	body := ""
	if m.Body != nil {
		body = *m.Body
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		log.Printf(`{"stream":"error","event":"bad_message","err":%q}`, err.Error())
		return true // poison message: drop rather than loop
	}
	detail := "null"
	if len(env.Detail) > 0 {
		detail = string(env.Detail)
	}
	log.Printf(`{"stream":"system","event":"handled","source":%q,"detail_type":%q,"detail":%s}`,
		env.Source, env.DetailType, detail)
	// TODO: real work per detail-type (e.g. call provider connector, update
	// mission status, enqueue notification). Demo logs and acks.
	return true
}
