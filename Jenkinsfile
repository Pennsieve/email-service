#!groovy

// CI for the email-service: the Go queue lambda (test, publish artifact, deploy
// via service-deploy) and the Scala producer client in client-scala/ (test +
// publish to CodeArtifact).
//
// Scala client versioning (com.pennsieve:email-client-scala), published to AWS
// CodeArtifact:
//   * main-branch build         -> publish <build>-<commit> (e.g. 31-3d9f63b),
//                                   the same scheme as the other Pennsieve clients.
//   * Tag build (vX.Y.Z)        -> publish release X.Y.Z.
//   * other branches            -> test only, no publish.
//
// No snapshots: CodeArtifact versions are immutable, so every publish is a
// unique release version, and consumers pin one.
// No Jenkins credential is involved: sbt gets a short-lived CodeArtifact token
// with the agent's instance role (see client-scala/build.sbt).

// scalaReleaseVersion returns "X.Y.Z" for a vX.Y.Z tag build, null otherwise.
def scalaReleaseVersion(tagName) {
  if (tagName?.trim() && tagName ==~ /^v\d+\.\d+\.\d+$/) {
    return tagName.substring(1) // strip leading 'v'
  }
  return null
}

ansiColor('xterm') {
  node('executor') {

  checkout scm

  def authorName  = sh(returnStdout: true, script: 'git --no-pager show --format="%an" --no-patch')
  def isMain    = env.BRANCH_NAME == "main"
  def serviceName = env.JOB_NAME.tokenize("/")[1]
  def isRealService = serviceName != "template-serverless-service"

  def commitHash  = sh(returnStdout: true, script: 'git rev-parse HEAD | cut -c-7').trim()
  def imageTag    = "${env.BUILD_NUMBER}-${commitHash}"

  def scalaVersion = scalaReleaseVersion(env.TAG_NAME) ?: (isMain ? imageTag : null)

  try {
    stage("Run Tests") {
      try {
        sh "IMAGE_TAG=${imageTag} make test-ci"
      } finally {
        sh "make docker-clean"
      }
    }

    // Test the Scala client against the shared wire-contract fixtures.
    stage("Test Scala client") {
      dir("client-scala") {
        sh "sbt -batch test"
      }
    }

    // Publish the Scala client: main as <build>-<commit>, a vX.Y.Z tag as X.Y.Z.
    if (scalaVersion) {
      stage("Publish Scala client") {
        dir("client-scala") {
          sh "sbt -batch -Dversion=${scalaVersion} publish"
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
