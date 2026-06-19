package com.pennsieve.email.client

import io.circe.syntax._
import software.amazon.awssdk.services.sqs.SqsClient
import software.amazon.awssdk.services.sqs.model.SendMessageRequest

import scala.util.{Failure, Success, Try}

/** Producer-side client for the Pennsieve email-service. Builds an EmailRequest
  * and enqueues it on the send queue; the consumer Lambda renders and delivers.
  *
  * @param sqs      an AWS SDK v2 SqsClient
  * @param queueUrl URL of the email-service send queue for the target environment
  */
class EmailClient(sqs: SqsClient, queueUrl: String) {

  /** Validate and enqueue an email request. Returns the SQS message id on
    * success. Delivery is asynchronous — this returns once the message is on
    * the queue.
    */
  def send(request: EmailRequest): Try[String] =
    validate(request) match {
      case Some(err) => Failure(new IllegalArgumentException(err))
      case None =>
        Try {
          val resp = sqs.sendMessage(
            SendMessageRequest
              .builder()
              .queueUrl(queueUrl)
              .messageBody(request.asJson.noSpaces)
              .build()
          )
          resp.messageId()
        }
    }

  private def validate(r: EmailRequest): Option[String] =
    if (r.messageId.isEmpty) Some("email request is missing messageId")
    else if (r.recipients.isEmpty) Some(s"email request '${r.messageId}' has no recipients")
    else
      r.recipients.zipWithIndex.collectFirst {
        case (rcpt, i) if rcpt.email.isEmpty =>
          s"email request '${r.messageId}' recipient $i has no email address"
      }
}
