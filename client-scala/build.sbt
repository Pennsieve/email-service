// Scala producer client for the Pennsieve email-service.
//
// Builds an EmailRequest and enqueues it on the email-service SQS queue. The
// JSON wire contract is shared with the Go client; both test against the
// fixtures in ../contract/fixtures so the shape cannot drift.

ThisBuild / organization := "com.pennsieve"
ThisBuild / scalaVersion := "2.13.16"

// CI injects the version (sbt -Dversion=... publish): <build>-<commit> on main,
// X.Y.Z on a vX.Y.Z tag (see the Jenkinsfile). The fallback only applies to
// local builds.
ThisBuild / version := sys.props.get("version").getOrElse("0.0.0-SNAPSHOT")

// Pennsieve packages live in AWS CodeArtifact (pennsieve-cc), `maven` repository. Access is IAM: the build exchanges the active
// AWS credentials -- the Jenkins agent's instance role, or `assume-role
// non-prod admin` locally -- for a short-lived token. Nothing is stored.
// CODEARTIFACT_AUTH_TOKEN, if set, is used instead (e.g. inside Docker builds).
lazy val codeArtifactHost = "pennsieve-932986170183.d.codeartifact.us-east-1.amazonaws.com"
lazy val codeArtifactRepo = s"https://$codeArtifactHost/maven/maven/"
lazy val codeArtifactToken: String = sys.env.get("CODEARTIFACT_AUTH_TOKEN").getOrElse {
  import scala.sys.process._
  val cmd = Seq("aws", "codeartifact", "get-authorization-token", "--domain", "pennsieve",
    "--domain-owner", "932986170183", "--region", "us-east-1",
    "--query", "authorizationToken", "--output", "text")
  scala.util.Try(cmd.!!(ProcessLogger(_ => ())).trim).getOrElse {
    println("[warn] No CodeArtifact token: AWS credentials missing or not allowed. Run assume-role non-prod admin.")
    ""
  }
}

ThisBuild / credentials += Credentials("pennsieve/maven", codeArtifactHost, "aws", codeArtifactToken)
ThisBuild / resolvers += "pennsieve" at codeArtifactRepo
ThisBuild / publishTo := Some("pennsieve" at codeArtifactRepo)
ThisBuild / publishMavenStyle := true

// Publish the way Maven does (sbt-aether-deploy). sbt's own publisher writes
// neither maven-metadata.xml nor timestamped snapshots, so in CodeArtifact its
// releases stay Unfinished and its snapshots are rejected.
overridePublishSettings

lazy val circeVersion = "0.14.1"
lazy val awsV2Version  = "2.20.26"

lazy val `email-client-scala` = (project in file("."))
  .settings(
    name := "email-client-scala",
    // The shared contract fixtures live one directory up; expose them to tests.
    Test / unmanagedResourceDirectories += (ThisBuild / baseDirectory).value / ".." / "contract" / "fixtures",
    libraryDependencies ++= Seq(
      "io.circe"               %% "circe-core"    % circeVersion,
      "io.circe"               %% "circe-generic" % circeVersion,
      "io.circe"               %% "circe-parser"  % circeVersion,
      "software.amazon.awssdk" %  "sqs"           % awsV2Version,
      "org.scalatest"          %% "scalatest"     % "3.2.17" % Test
    )
  )
