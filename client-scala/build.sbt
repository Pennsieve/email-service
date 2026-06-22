// Scala producer client for the Pennsieve email-service.
//
// Builds an EmailRequest and enqueues it on the email-service SQS queue. The
// JSON wire contract is shared with the Go client; both test against the
// fixtures in ../contract/fixtures so the shape cannot drift.

ThisBuild / organization := "com.pennsieve"
ThisBuild / scalaVersion := "2.13.16"

// Coursier's HTTP handling against Nexus causes redirect loops on Jenkins
// ("Server redirected too many times"); the classic Ivy resolver does not.
// Mirrors pennsieve-api (set in both build.sbt and project/plugins.sbt).
ThisBuild / useCoursier := false

// CI always injects the version (sbt -Dversion=X.Y.Z publish): a vX.Y.Z tag
// publishes the release X.Y.Z; a main build publishes the next-minor -SNAPSHOT
// (see the Jenkinsfile). The fallback only applies to a bare local `sbt publish`.
ThisBuild / version := sys.props.get("version").getOrElse("0.0.0-SNAPSHOT")

// Publish to the Pennsieve Nexus, same as pennsieve-api. Credentials come from
// the environment (the Jenkins executor provides them) — no creds in the repo.
ThisBuild / credentials += Credentials(
  "Sonatype Nexus Repository Manager",
  "nexus.pennsieve.cc",
  sys.env.getOrElse("PENNSIEVE_NEXUS_USER", ""),
  sys.env.getOrElse("PENNSIEVE_NEXUS_PW", "")
)

ThisBuild / publishTo := {
  val nexus = "https://nexus.pennsieve.cc/repository"
  if (isSnapshot.value) Some("Nexus Realm" at s"$nexus/maven-snapshots")
  else Some("Nexus Realm" at s"$nexus/maven-releases")
}
ThisBuild / publishMavenStyle := true

ThisBuild / resolvers ++= Seq(
  "Pennsieve Releases" at "https://nexus.pennsieve.cc/repository/maven-releases",
  "Pennsieve Snapshots" at "https://nexus.pennsieve.cc/repository/maven-snapshots"
)

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
