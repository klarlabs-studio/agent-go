// Package messaging provides message queue tools for agent-go.
//
// This pack includes tools for message queue operations:
//   - mq_publish: Publish a message to a topic/queue
//   - mq_subscribe: Subscribe to messages from a topic/queue
//   - mq_ack: Acknowledge message processing
//   - mq_nack: Negative acknowledge (requeue) a message
//   - mq_list_queues: List available queues
//   - mq_queue_info: Get queue statistics and metadata
//   - mq_purge: Purge all messages from a queue
//
// Supports RabbitMQ, Apache Kafka, AWS SQS, Google Pub/Sub, and NATS.
package messaging

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the messaging tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("messaging").
		WithDescription("Message queue tools for pub/sub and queue operations").
		WithVersion("0.1.0").
		AddTools(
			mqPublish(),
			mqSubscribe(),
			mqAck(),
			mqNack(),
			mqListQueues(),
			mqQueueInfo(),
			mqPurge(),
		).
		AllowInState(agent.StateExplore, "mq_list_queues", "mq_queue_info").
		AllowInState(agent.StateAct, "mq_publish", "mq_subscribe", "mq_ack", "mq_nack", "mq_list_queues", "mq_queue_info", "mq_purge").
		Build()
}

func mqPublish() tool.Tool {
	return tool.NewBuilder("mq_publish").
		WithDescription("Publish a message to a topic or queue").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func mqSubscribe() tool.Tool {
	return tool.NewBuilder("mq_subscribe").
		WithDescription("Subscribe and receive messages from a topic or queue").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func mqAck() tool.Tool {
	return tool.NewBuilder("mq_ack").
		WithDescription("Acknowledge successful message processing").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func mqNack() tool.Tool {
	return tool.NewBuilder("mq_nack").
		WithDescription("Negative acknowledge a message for requeue or dead-letter").
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func mqListQueues() tool.Tool {
	return tool.NewBuilder("mq_list_queues").
		WithDescription("List available queues and topics").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func mqQueueInfo() tool.Tool {
	return tool.NewBuilder("mq_queue_info").
		WithDescription("Get queue statistics and metadata").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func mqPurge() tool.Tool {
	return tool.NewBuilder("mq_purge").
		WithDescription("Purge all messages from a queue").
		Destructive().
		MustBuild()
}
