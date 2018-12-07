
def kubeConfigRoot = "/home/jenkins/.kube"
/*
def buildBuildSteps(Map myParams) {
    return {
        timestamps {
            timeout(time: 15) {
                withEnv([
                "DEPLOYMENTNAMESPACE=${myParams.TESTNAMESPACE}-${env.GIT_COMMIT}",
                "DOCKERNAMESPACE=${myParams.DOCKERNAMESPACE}",
                "IMAGETAG=jenkins-test",
                "LONG=${myParams.LONG ? 1 : 0}",
                "TESTOPTIONS=${myParams.TESTOPTIONS}",
                ]) {
                    sh "make clean"
                    sh "make"
                    sh "make run-unit-tests"
                    sh "make docker-test"
                }
            }
        }
    }
}

def buildTestSteps(Map myParams, String kubeConfigRoot, String kubeconfig) {
    return {
        timestamps {
            timeout(time: myParams.LONG ? 180 : 30) {
                withCredentials([
                    string(credentialsId: 'ENTERPRISEIMAGE', variable: 'DEFAULTENTERPRISEIMAGE'),
                    string(credentialsId: 'ENTERPRISELICENSE', variable: 'DEFAULTENTERPRISELICENSE'),
                ]) { 
                    withEnv([
                    "CLEANDEPLOYMENTS=1",
                    "DEPLOYMENTNAMESPACE=${myParams.TESTNAMESPACE}-${env.GIT_COMMIT}",
                    "DOCKERNAMESPACE=${myParams.DOCKERNAMESPACE}",
                    "ENTERPRISEIMAGE=${myParams.ENTERPRISEIMAGE}",
                    "ENTERPRISELICENSE=${myParams.ENTERPRISELICENSE}",
                    "ARANGODIMAGE=${myParams.ARANGODIMAGE}",
                    "IMAGETAG=jenkins-test-weekly",
                    "KUBECONFIG=${kubeConfigRoot}/${kubeconfig}",
                    "LONG=${myParams.LONG ? 1 : 0}",
                    "TESTOPTIONS=${myParams.TESTOPTIONS}",
                    ]) {
                        sh "make run-tests"
                    }
                }
            }
        }
    }
}

def buildCleanupSteps(Map myParams, String kubeConfigRoot, String kubeconfig) {
    return {
        stage(kubeconfig) {
            steps {
                timestamps {
                    timeout(time: 15) {
                        withEnv([
                            "DEPLOYMENTNAMESPACE=${myParams.TESTNAMESPACE}-${env.GIT_COMMIT}",
                            "DOCKERNAMESPACE=${myParams.DOCKERNAMESPACE}",
                            "KUBECONFIG=${kubeConfigRoot}/${kubeconfig}",
                        ]) {
                            sh "./scripts/collect_logs.sh ${env.DEPLOYMENTNAMESPACE} ${kubeconfig}" 
                            archive includes: 'logs/*'
                            sh "make cleanup-tests"
                        }
                    }
                }
            }
        }
    }
}*/

def buildTestSteps(String platformStr, String imageStr, String editionStr) {
    def tasks = [:]

    def platforms = platformStr.split(',')
    def images = imageStr.split(',')
    def editions = editionStr.split(',')

    configFileProvider([
        configFile(fileId: 'arangodb-images', variable: 'ARANGODB_IMAGES_FILE'),
        configFile(fileId: 'k8s-cluster', variable: 'ARANGODB_K8S_FILE')
    ]) {
        def k8s = readJSON file: env.ARANGODB_IMAGES_FILE
        def arango = readJSON file: env.ARANGODB_IMAGES_FILE

        for (p in platforms) {
            for (i in images) {
                for (e in editions) {
                    if (k8s.containsKey(p)) {
                        def platform = k8s[p]

                        if (arango.containsKey(i)) {
                            def version = arango[i]

                            if (version.containsKey(e)) {
                                def image = version(e)
                                def stepName = "${p}-${i}-${e}"

                                def env = [:]
                                if (image.containsKey('env')) {

                                }

                                tasks[stepName] = {
                                    stage(stepName) {
                                        steps {
                                            script {
                                                echo "${stepName}"
                                            }
                                        }
                                    }
                                }


                            } else {
                                println("Unkown edition ${e}")
                            }
                        } else {
                            println("Unknown image ${i}")
                        }
                    } else {
                        println("Unknown platform ${p}")
                    }
                }
            }
        }
    }

    return tasks
}

def buildCleanupSteps(String platform, String image, String edition) {

}

pipeline {
    options {
        buildDiscarder(logRotator(daysToKeepStr: '7', numToKeepStr: '10'))
        lock resource: 'kube-arangodb'
    }
    agent any
    parameters {
      /*booleanParam(name: 'LONG', defaultValue: false, description: 'Execute long running tests')
      string(name: 'DOCKERNAMESPACE', defaultValue: 'arangodb', description: 'DOCKERNAMESPACE sets the docker registry namespace in which the operator docker image will be pushed', )
      string(name: 'KUBECONFIGS', defaultValue: 'kube-ams1,scw-183a3b', description: 'KUBECONFIGS is a comma separated list of Kubernetes configuration files (relative to /home/jenkins/.kube) on which the tests are run', )
      string(name: 'TESTNAMESPACE', defaultValue: 'jenkins', description: 'TESTNAMESPACE sets the kubernetes namespace to ru tests in (this must be short!!)', )
      string(name: 'ENTERPRISEIMAGE', defaultValue: '', description: 'ENTERPRISEIMAGE sets the docker image used for enterprise tests', )
      string(name: 'ARANGODIMAGE', defaultValue: '', description: 'ARANGODIMAGE sets the docker image used for tests (except enterprise and update tests)', )
      string(name: 'ENTERPRISELICENSE', defaultValue: '', description: 'ENTERPRISELICENSE sets the enterprise license key for enterprise tests', )*/
      string(name: 'images', defaultValue: '')
      string(name: 'platforms', defaultValue: '')
      string(name: 'editions', defaultValue: '')
    }
    stages {
        stage('Test') {
            steps {
                script {
                    def tasks = buildTestSteps(params.platforms, params.images, params.editions)
                    parallel tasks
                }
            }
        }
    }


}
