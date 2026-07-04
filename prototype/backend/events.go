// Amazon EventBridge publisher (managed messaging).
// Emits domain events onto the custom bus (EVENT_BUS_NAME) for async workers
// (dispatch, notification) subscribed via SQS. No-ops when unconfigured.
package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

var (
	ebClient *eventbridge.Client
	eventBus = getenv("EVENT_BUS_NAME", "")
)

func initEvents(cfg aws.Config) {
	ebClient = eventbridge.NewFromConfig(cfg)
}

func publishEvent(ctx context.Context, detailType string, detail map[string]any) {
	if ebClient == nil || eventBus == "" {
		return
	}
	b, _ := json.Marshal(detail)
	entry := ebtypes.PutEventsRequestEntry{
		EventBusName: aws.String(eventBus),
		Source:       aws.String("insucar.api"),
		DetailType:   aws.String(detailType),
		Detail:       aws.String(string(b)),
		Time:         aws.Time(time.Now()),
	}
	if _, err := ebClient.PutEvents(ctx, &eventbridge.PutEventsInput{Entries: []ebtypes.PutEventsRequestEntry{entry}}); err != nil {
		log.Printf(`{"stream":"error","event":"eventbridge_publish_failed","detail_type":%q,"err":%q}`, detailType, err.Error())
	}
}
