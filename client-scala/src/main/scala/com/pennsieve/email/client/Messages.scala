package com.pennsieve.email.client

import io.circe.Json

/** Typed builders that construct an EmailRequest for a specific messageId. Each
  * owns the exact messageId and context keys the template expects (see the
  * email-templates repo's template-variables.json). These mirror the Go
  * client's builders so the two languages produce identical wire payloads.
  *
  * For a template without a typed builder yet, use Messages.message(...).
  */
object Messages {

  /** Untyped escape hatch: build a request for an arbitrary messageId. Prefer a
    * typed builder when one exists.
    */
  def message(messageId: String, to: Recipient, context: Map[String, Json] = Map.empty): EmailRequest =
    EmailRequest(messageId = messageId, recipients = List(to), context = context)

  private def str(kvs: (String, String)*): Map[String, Json] =
    kvs.map { case (k, v) => k -> Json.fromString(v) }.toMap

  def datasetPublicationAccepted(
      to: Recipient,
      datasetName: String,
      reviewerName: String,
      date: String
  ): EmailRequest =
    message(
      "dataset-publication-accepted",
      to,
      str("datasetName" -> datasetName, "reviewerName" -> reviewerName, "date" -> date)
    )

  def changeOfDatasetOwner(
      to: Recipient,
      datasetName: String,
      datasetNodeId: String,
      host: String,
      organizationName: String,
      organizationNodeId: String,
      previousOwnerName: String
  ): EmailRequest =
    message(
      "change-of-dataset-owner",
      to,
      str(
        "datasetName"        -> datasetName,
        "datasetNodeId"      -> datasetNodeId,
        "host"               -> host,
        "organizationName"   -> organizationName,
        "organizationNodeId" -> organizationNodeId,
        "previousOwnerName"  -> previousOwnerName
      )
    )

  def addedToTeam(
      to: Recipient,
      administrator: String,
      teamName: String,
      host: String,
      organizationNodeId: String
  ): EmailRequest =
    message(
      "added-to-team",
      to,
      str(
        "administrator"      -> administrator,
        "teamName"           -> teamName,
        "host"               -> host,
        "organizationNodeId" -> organizationNodeId
      )
    )

  def rehydrationComplete(
      to: Recipient,
      datasetID: String,
      datasetVersionID: String,
      rehydrationLocation: String,
      awsRegion: String
  ): EmailRequest =
    message(
      "rehydration-complete",
      to,
      str(
        "DatasetID"           -> datasetID,
        "DatasetVersionID"    -> datasetVersionID,
        "RehydrationLocation" -> rehydrationLocation,
        "AWSRegion"           -> awsRegion
      )
    )

  def datasetProposalSubmitted(
      to: Recipient,
      authorName: String,
      authorEmail: String,
      proposalTitle: String,
      workspaceName: String,
      workspaceNodeId: String,
      appURL: String
  ): EmailRequest =
    message(
      "dataset-proposal-submitted",
      to,
      str(
        "AuthorName"      -> authorName,
        "AuthorEmail"     -> authorEmail,
        "ProposalTitle"   -> proposalTitle,
        "WorkspaceName"   -> workspaceName,
        "WorkspaceNodeId" -> workspaceNodeId,
        "AppURL"          -> appURL
      )
    )
}
