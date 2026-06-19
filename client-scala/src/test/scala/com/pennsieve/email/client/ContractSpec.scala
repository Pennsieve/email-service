package com.pennsieve.email.client

import io.circe.parser
import io.circe.syntax._
import org.scalatest.funsuite.AnyFunSuite

import scala.io.Source

/** Locks the Scala client to the shared wire contract in ../contract/fixtures
  * (exposed on the test classpath via build.sbt). The Go client tests against
  * the same files, so a shape change in either language fails CI.
  */
class ContractSpec extends AnyFunSuite {

  private def fixture(name: String): String = {
    val src = Source.fromResource(name)
    try src.mkString
    finally src.close()
  }

  private val fixtures = List(
    "minimal.json",
    "with-organization.json",
    "with-dedupe-id.json",
    "multi-recipient.json"
  )

  test("every fixture decodes and re-encodes structurally unchanged") {
    fixtures.foreach { name =>
      val raw     = fixture(name)
      val parsed  = parser.parse(raw).fold(throw _, identity)
      val request = parsed.as[EmailRequest].fold(throw _, identity)

      assert(request.messageId.nonEmpty, s"$name: messageId")
      assert(request.recipients.nonEmpty, s"$name: recipients")

      // Compare parsed JSON (key order / whitespace insensitive).
      assert(request.asJson === parsed, s"$name: re-encoding must match the fixture")
    }
  }

  test("builder produces the documented wire shape") {
    val req = Messages.datasetPublicationAccepted(
      Recipient("Alice", "alice@example.com"),
      datasetName = "My Dataset",
      reviewerName = "Bob",
      date = "2026-06-18"
    )
    val minimal = parser.parse(fixture("minimal.json")).fold(throw _, identity)
    assert(req.asJson === minimal)

    val withOrg    = req.withOrganization(367L)
    val orgFixture = parser.parse(fixture("with-organization.json")).fold(throw _, identity)
    assert(withOrg.asJson === orgFixture)
  }

  test("dedupeId is omitted when not set and present when set") {
    val without = Messages.addedToTeam(Recipient("C", "c@x.com"), "admin", "team", "host", "N:org:x")
    assert(!without.asJson.asObject.exists(_.contains("dedupeId")), "dedupeId must be absent")

    val withId = without.withDedupeId("evt-1")
    assert(withId.asJson.asObject.exists(_.contains("dedupeId")), "dedupeId must be present")
  }
}
