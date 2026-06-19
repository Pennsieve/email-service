// Resolve meta-build (plugin) artifacts through the Pennsieve Nexus proxy with
// credentials, exactly like pennsieve-api. Without these, sbt hits Nexus
// unauthenticated, follows redirects without sending auth, and fails on Jenkins
// with "java.net.ProtocolException: Server redirected too many times (20)".
ThisBuild / resolvers ++= Seq(
  "pennsieve-maven-proxy" at "https://nexus.pennsieve.cc/repository/maven-public",
  Resolver.url("pennsieve-ivy-proxy", url("https://nexus.pennsieve.cc/repository/ivy-public/"))(Patterns("[organization]/[module]/(scala_[scalaVersion]/)(sbt_[sbtVersion]/)[revision]/[type]s/[artifact](-[classifier]).[ext]")),
)

ThisBuild / credentials += Credentials(
  "Sonatype Nexus Repository Manager",
  "nexus.pennsieve.cc",
  sys.env.getOrElse("PENNSIEVE_NEXUS_USER", ""),
  sys.env.getOrElse("PENNSIEVE_NEXUS_PW", "")
)

// Coursier's HTTP handling against Nexus causes redirect loops on Jenkins; the
// classic Ivy resolver does not. Mirrors pennsieve-api.
ThisBuild / useCoursier := false

addSbtPlugin("com.typesafe.sbt" % "sbt-git" % "1.0.0")
