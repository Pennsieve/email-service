// Scala producer client for the Pennsieve email-service.
//
// Builds an EmailRequest and enqueues it on the email-service SQS queue. The
// JSON wire contract is shared with the Go client; both test against the
// fixtures in ../contract/fixtures so the shape cannot drift.

ThisBuild / organization := "com.pennsieve"
ThisBuild / scalaVersion := "2.13.12"

lazy val circeVersion = "0.14.1"
lazy val awsV2Version  = "2.20.26"

lazy val `email-client-scala` = (project in file("."))
  .settings(
    name := "email-client-scala",
    // The shared contract fixtures live one directory up; expose them to tests.
    Test / unmanagedResourceDirectories += (ThisBuild / baseDirectory).value / ".." / "contract" / "fixtures",
    libraryDependencies ++= Seq(
      "io.circe"               %% "circe-core"     % circeVersion,
      "io.circe"               %% "circe-generic"  % circeVersion,
      "io.circe"               %% "circe-parser"   % circeVersion,
      "software.amazon.awssdk" %  "sqs"            % awsV2Version,
      "org.scalatest"          %% "scalatest"      % "3.2.17" % Test
    )
  )
