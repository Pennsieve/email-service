#!groovy

// CI for the email-service: the Go queue lambda (test, publish artifact, deploy
// via service-deploy) and the Scala producer client in client-scala/ (test +
// publish to Nexus).
//
// Scala client versioning (com.pennsieve:email-client-scala) — releases are
// intentional, driven by git tags:
//   * Tag build (vX.Y.Z)        -> publish release X.Y.Z to maven-releases.
//   * main-branch build         -> publish next-minor-SNAPSHOT to maven-snapshots
//                                   (e.g. latest tag v1.2.0 -> 1.3.0-SNAPSHOT).
//   * other branches            -> test only, no publish.
//
// Nexus credentials are bound from the Jenkins credential 'pennsieve-nexus-ci-login'
// around every sbt invocation (resolve + publish), matching pennsieve-api.
// Without them sbt hits Nexus unauthenticated and fails with
// "Server redirected too many times".

def pennsieveNexusCreds = usernamePassword(
    credentialsId: 'pennsieve-nexus-ci-login',
    usernameVariable: 'PENNSIEVE_NEXUS_USER',
    passwordVariable: 'PENNSIEVE_NEXUS_PW'
)

// scalaClientVersion computes the version to publish:
//   - on a vX.Y.Z tag  -> "X.Y.Z"            (release)
//   - otherwise        -> "X.(Y+1).0-SNAPSHOT" from the latest vX.Y.Z tag,
//                         or "0.1.0-SNAPSHOT" if there are no tags yet.
def scalaClientVersion(tagName) {
  if (tagName?.trim() && tagName ==~ /^v\d+\.\d+\.\d+$/) {
    return tagName.substring(1) // strip leading 'v'
  }
  def latest = sh(
    returnStdout: true,
    script: "git tag --list 'v*' --sort=-v:refname | head -1"
  ).trim()
  if (!latest) {
    return "0.1.0-SNAPSHOT"
  }
  def m = (latest =~ /^v(\d+)\.(\d+)\.(\d+)$/)
  def major = m[0][1] as int
  def minor = m[0][2] as int
  return "${major}.${minor + 1}.0-SNAPSHOT"
}

ansiColor('xterm') {
  node('executor') {

  checkout scm

  def authorName  = sh(returnStdout: true, script: 'git --no-pager show --format="%an" --no-patch')
  def isMain    = env.BRANCH_NAME == "main"
  def isTag     = (env.TAG_NAME?.trim()) ? true : false
  def serviceName = env.JOB_NAME.tokenize("/")[1]
  def isRealService = serviceName != "template-serverless-service"

  def commitHash  = sh(returnStdout: true, script: 'git rev-parse HEAD | cut -c-7').trim()
  def imageTag    = "${env.BUILD_NUMBER}-${commitHash}"

  def scalaVersion = scalaClientVersion(env.TAG_NAME)

  try {
    stage("Run Tests") {
      try {
        sh "IMAGE_TAG=${imageTag} make test-ci"
      } finally {
        sh "make docker-clean"
      }
    }

    // Test the Scala client against the shared wire-contract fixtures.
    // Needs Nexus creds: sbt resolves dependencies (and plugins) from Nexus.
    stage("Test Scala client") {
      withCredentials([pennsieveNexusCreds]) {
        dir("client-scala") {
          sh "sbt -batch test"
        }
      }
    }

    // Publish the Scala client. A vX.Y.Z tag publishes the release X.Y.Z; a main
    // build publishes the next-minor -SNAPSHOT. Other branches don't publish.
    if (isTag || isMain) {
      stage("Publish Scala client") {
        withCredentials([pennsieveNexusCreds]) {
          dir("client-scala") {
            sh "sbt -batch -Dversion=${scalaVersion} publish"
          }
        }
      }
    }

    // The Go lambda is built/published/deployed only on main (a tag is a library
    // release, not a service deploy).
    if (isMain) {
      stage ('Build and Push') {
        sh "IMAGE_TAG=${imageTag} make publish"
      }

      if(isRealService) {
        stage("Deploy") {
            build job: "service-deploy/pennsieve-non-prod/us-east-1/dev-vpc-use1/dev/${serviceName}",
            parameters: [
                string(name: 'IMAGE_TAG', value: imageTag),
                string(name: 'TERRAFORM_ACTION', value: 'apply')
            ]
        }
      }
    }
  } catch (e) {
    slackSend(color: '#b20000', message: "FAILED: Job '${env.JOB_NAME} [${env.BUILD_NUMBER}]' (${env.BUILD_URL}) by ${authorName}")
    throw e
  }

  slackSend(color: '#006600', message: "SUCCESSFUL: Job '${env.JOB_NAME} [${env.BUILD_NUMBER}]' (${env.BUILD_URL}) by ${authorName}")
  }
}
