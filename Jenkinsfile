#!groovy

// CI for the email-service: the Go queue lambda (test, publish artifact, deploy
// via service-deploy) and the Scala producer client in client-scala/ (test, and
// publish to Nexus on main).
//
// RELEASE_VERSION (optional) publishes the Scala client as a release artifact
// (com.pennsieve:email-client-scala) at that version; left blank, main builds
// publish a -SNAPSHOT. Nexus credentials are bound from the Jenkins credential
// 'pennsieve-nexus-ci-login' around every sbt invocation (resolve + publish),
// matching pennsieve-api. Without them sbt hits Nexus unauthenticated and fails
// with "Server redirected too many times".

def pennsieveNexusCreds = usernamePassword(
    credentialsId: 'pennsieve-nexus-ci-login',
    usernameVariable: 'PENNSIEVE_NEXUS_USER',
    passwordVariable: 'PENNSIEVE_NEXUS_PW'
)

properties([
  parameters([
    string(
      name: 'RELEASE_VERSION',
      defaultValue: '',
      description: 'If set (e.g. 1.2.3), publish the Scala client as a release at this version. Blank = SNAPSHOT.'
    )
  ])
])

ansiColor('xterm') {
  node('executor') {

  checkout scm

  def authorName  = sh(returnStdout: true, script: 'git --no-pager show --format="%an" --no-patch')
  def isMain    = env.BRANCH_NAME == "main"
  def serviceName = env.JOB_NAME.tokenize("/")[1]
  def isRealService = serviceName != "template-serverless-service"

  def commitHash  = sh(returnStdout: true, script: 'git rev-parse HEAD | cut -c-7').trim()
  def imageTag    = "${env.BUILD_NUMBER}-${commitHash}"

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

    if(isMain) {
      stage ('Build and Push') {
        sh "IMAGE_TAG=${imageTag} make publish"
      }

      // Publish the Scala client to Nexus. With RELEASE_VERSION set it's a
      // release; otherwise a SNAPSHOT (version defaults to bootstrap-SNAPSHOT).
      stage("Publish Scala client") {
        withCredentials([pennsieveNexusCreds]) {
          dir("client-scala") {
            if (params.RELEASE_VERSION?.trim()) {
              sh "sbt -batch -Dversion=${params.RELEASE_VERSION} publish"
            } else {
              sh "sbt -batch publish"
            }
          }
        }
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
