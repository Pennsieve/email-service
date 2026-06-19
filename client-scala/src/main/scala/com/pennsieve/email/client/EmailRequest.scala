package com.pennsieve.email.client

import io.circe.{Decoder, Encoder, Json}
import io.circe.syntax._

/** A single destination for an email. */
final case class Recipient(name: String, email: String)

object Recipient {
  implicit val encoder: Encoder[Recipient] = Encoder.forProduct2("name", "email")(r => (r.name, r.email))
  implicit val decoder: Decoder[Recipient] = Decoder.forProduct2("name", "email")(Recipient.apply)
}

/** The JSON payload enqueued on the email-service SQS queue. This is the wire
  * contract shared with the Go client (see ../contract). The email-service
  * consumer renders the template and delivers via SES — producers never touch
  * SES.
  *
  * `context` is heterogeneous (organizationId is a number, most keys are
  * strings), so it is modeled as Map[String, Json]. `dedupeId` is omitted from
  * the JSON when empty, matching the Go client's `omitempty`.
  */
final case class EmailRequest(
    messageId: String,
    recipients: List[Recipient],
    context: Map[String, Json] = Map.empty,
    dedupeId: Option[String] = None
) {

  /** Set the organization id so the consumer resolves an org-branded template
    * (custom/O{id}/) with fallback to the default.
    */
  def withOrganization(organizationId: Long): EmailRequest =
    copy(context = context + ("organizationId" -> Json.fromLong(organizationId)))

  /** Provide an explicit idempotency id (preferred over the content hash). */
  def withDedupeId(id: String): EmailRequest =
    copy(dedupeId = Some(id))

  /** Override the template's default subject for this send. */
  def withSubject(subject: String): EmailRequest =
    copy(context = context + ("subject" -> Json.fromString(subject)))
}

object EmailRequest {

  // Encoder drops dedupeId when None (matches Go's omitempty) and always emits
  // messageId, recipients, and context (context as {} when empty).
  implicit val encoder: Encoder[EmailRequest] = Encoder.instance { r =>
    val base = List(
      "messageId"  -> Json.fromString(r.messageId),
      "recipients" -> r.recipients.asJson,
      "context"    -> Json.fromJsonObject(io.circe.JsonObject.fromMap(r.context))
    )
    val withDedupe = r.dedupeId match {
      case Some(id) => ("dedupeId" -> Json.fromString(id)) :: base
      case None     => base
    }
    Json.obj(withDedupe: _*)
  }

  implicit val decoder: Decoder[EmailRequest] = Decoder.instance { c =>
    for {
      messageId  <- c.get[String]("messageId")
      recipients <- c.get[List[Recipient]]("recipients")
      context    <- c.getOrElse[Map[String, Json]]("context")(Map.empty)
      dedupeId   <- c.get[Option[String]]("dedupeId")
    } yield EmailRequest(messageId, recipients, context, dedupeId)
  }
}
