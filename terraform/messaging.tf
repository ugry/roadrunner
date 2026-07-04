########################################
# Managed messaging — Amazon EventBridge + SNS + SQS
#
# Replaces self-managed NATS JetStream. Domain events flow onto a custom
# EventBridge bus; durable work queues (SQS) with dead-letter queues fan out
# to the async workers (dispatch, notification). An SNS topic is kept for
# pub/sub fan-out (SMS already uses SNS directly).
########################################

# --- EventBridge custom bus (domain events) ------------------------------
resource "aws_cloudwatch_event_bus" "events" {
  name = "insucar-${var.environment}"
}

# --- SNS topic (fan-out) -------------------------------------------------
resource "aws_sns_topic" "events" {
  name = "insucar-${var.environment}-events"
}

# --- SQS work queues + dead-letter queues --------------------------------
locals {
  queues = ["dispatch", "notification"]
}

resource "aws_sqs_queue" "dlq" {
  for_each                  = toset(local.queues)
  name                      = "insucar-${var.environment}-${each.value}-dlq"
  message_retention_seconds = 1209600 # 14 days
  sqs_managed_sse_enabled   = true
}

resource "aws_sqs_queue" "work" {
  for_each                   = toset(local.queues)
  name                       = "insucar-${var.environment}-${each.value}"
  visibility_timeout_seconds = 60
  sqs_managed_sse_enabled    = true
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq[each.value].arn
    maxReceiveCount     = 5
  })
}

# Route matching EventBridge events into the dispatch queue as an example
# (extend with per-detail-type rules as the service set lands).
resource "aws_cloudwatch_event_rule" "dispatch" {
  name           = "insucar-${var.environment}-dispatch"
  event_bus_name = aws_cloudwatch_event_bus.events.name
  event_pattern = jsonencode({
    "detail-type" = ["case.dispatch.requested"]
  })
}

resource "aws_cloudwatch_event_target" "dispatch_to_sqs" {
  rule           = aws_cloudwatch_event_rule.dispatch.name
  event_bus_name = aws_cloudwatch_event_bus.events.name
  arn            = aws_sqs_queue.work["dispatch"].arn
}

# Allow EventBridge to deliver to the dispatch queue.
resource "aws_sqs_queue_policy" "dispatch_from_eventbridge" {
  queue_url = aws_sqs_queue.work["dispatch"].id
  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Effect    = "Allow",
      Principal = { Service = "events.amazonaws.com" },
      Action    = "sqs:SendMessage",
      Resource  = aws_sqs_queue.work["dispatch"].arn,
      Condition = { ArnEquals = { "aws:SourceArn" = aws_cloudwatch_event_rule.dispatch.arn } }
    }]
  })
}

output "event_bus_name" { value = aws_cloudwatch_event_bus.events.name }
output "sns_events_topic_arn" { value = aws_sns_topic.events.arn }
output "sqs_queue_urls" {
  value = { for k, q in aws_sqs_queue.work : k => q.url }
}
