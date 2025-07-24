/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/uselagoon/aergia-controller/test/utils"
)

const (
	namespace = "aergia-controller-system"
	timeout   = "600s"
)

func init() {
	kindIP = os.Getenv("KIND_NODE_IP")
}

var (
	duration = 600 * time.Second
	interval = 1 * time.Second

	kindIP string

	metricLabels = []string{
		"aergia_allowed_requests",
		"aergia_blocked_by_block_list",
		"aergia_cli_idling_events",
		"aergia_idling_events",
		"aergia_no_namespace",
		"aergia_unidling_events",
		"aergia_verification_requests",
		"aergia_verification_required_requests",
	}
)

var _ = ginkgo.Describe("controller", ginkgo.Ordered, func() {
	ginkgo.BeforeAll(func() {

		ginkgo.By("creating manager namespace")
		cmd := exec.Command(utils.Kubectl(), "create", "ns", namespace)
		_, _ = utils.Run(cmd)

		ginkgo.By("remove ingress-nginx custom backend")
		cmd = exec.Command(utils.Kubectl(), "-n", "ingress-nginx", "patch", "deployment",
			"ingress-nginx-controller", "--type=json",
			"-p=[{'op': 'remove', 'path': '/spec/template/spec/containers/0/args/10'}]",
		)
		_, _ = utils.Run(cmd)

		// when running a re-test, it is best to make sure the old namespace doesn't exist
		ginkgo.By("removing existing test resources")

		// remove the example namespace
		cmd = exec.Command(utils.Kubectl(), "delete", "ns", "example-nginx")
		_, _ = utils.Run(cmd)
	})

	// comment to prevent cleaning up controller namespace and local services
	ginkgo.AfterAll(func() {
		ginkgo.By("remove ingress-nginx custom backend")
		cmd := exec.Command(utils.Kubectl(), "-n", "ingress-nginx", "patch", "deployment",
			"ingress-nginx-controller", "--type=json",
			"-p=[{'op': 'remove', 'path': '/spec/template/spec/containers/0/args/10'}]",
		)
		_, _ = utils.Run(cmd)

		ginkgo.By("stop metrics consumer")
		utils.StopMetricsConsumer()

		// remove the example namespace
		cmd = exec.Command(utils.Kubectl(), "delete", "ns", "example-nginx")
		_, _ = utils.Run(cmd)

		ginkgo.By("removing manager namespace")
		cmd = exec.Command(utils.Kubectl(), "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	ginkgo.Context("Operator", func() {
		ginkgo.It("should run successfully", func() {
			// start tests
			var controllerPodName string
			var err error

			// projectimage stores the name of the image used in the example
			var projectimage = "example.com/aergia-controller:v0.0.1"

			ginkgo.By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

			ginkgo.By("loading the the manager(Operator) image on Kind")
			err = utils.LoadImageToKindClusterWithName(projectimage)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

			ginkgo.By("deploying the controller-manager")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

			ginkgo.By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name

				cmd = exec.Command(utils.Kubectl(), "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				gomega.ExpectWithOffset(2, controllerPodName).Should(gomega.ContainSubstring("controller-manager"))

				cmd = exec.Command(utils.Kubectl(), "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				status, err := utils.Run(cmd)
				gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			gomega.EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(gomega.Succeed())

			ginkgo.By("patch ingress-nginx custom backend")
			cmd = exec.Command(utils.Kubectl(), "-n", "ingress-nginx", "patch", "deployment",
				"ingress-nginx-controller", "--type=json",
				"-p=[{'op': 'add', 'path': '/spec/template/spec/containers/0/args/-', 'value': '--default-backend-service=aergia-controller-system/aergia-controller-controller-manager-backend' }]",
			)
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

			time.Sleep(5 * time.Second)

			ginkgo.By("start metrics consumer")
			gomega.Expect(utils.StartMetricsConsumer()).To(gomega.Succeed())

			time.Sleep(30 * time.Second)

			ginkgo.By("creating a basic deployment")
			cmd = exec.Command(
				utils.Kubectl(),
				"apply",
				"-f",
				"test/e2e/testdata/example-nginx.yaml",
			)
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

			time.Sleep(10 * time.Second)

			// verify unidling by curl works
			ginkgo.By("triggering an unidle event")
			runCmd := fmt.Sprintf(`curl -s -I http://aergia.%s.nip.io/`, kindIP)
			output, err := utils.RunCommonsCommand(namespace, runCmd)
			gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
			err = utils.CheckStringContainsStrings(string(output), []string{"503 Service"})
			gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())

			time.Sleep(5 * time.Second)

			// verify that it eventually unidles via curl
			ginkgo.By("validating environment unidles")
			verifyUnidling := func() error {
				runCmd := fmt.Sprintf(`curl -s -I http://aergia.%s.nip.io/`, kindIP)
				output, err := utils.RunCommonsCommand(namespace, runCmd)
				gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
				err = utils.CheckStringContainsStrings(string(output), []string{"200 OK"})
				if err != nil {
					return err
				}
				return nil
			}
			gomega.EventuallyWithOffset(1, verifyUnidling, duration, interval).Should(gomega.Succeed())
			time.Sleep(5 * time.Second)
			// verify that force idling works, and that a 503 is eventually returned
			ginkgo.By("validating that force idling works")
			cmd = exec.Command(
				utils.Kubectl(), "patch", "namespace", "example-nginx",
				"--type", "merge",
				"-p", "{\"metadata\":{\"labels\":{\"idling.amazee.io/force-idled\":\"true\"}}}",
			)
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
			time.Sleep(5 * time.Second)
			ginkgo.By("wait for environment to idle")
			// check the pods in the environment disappear
			verifyForceIdlingPods := func() error {
				cmd = exec.Command(utils.Kubectl(), "get",
					"pods", "-l", "app=example-nginx",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", "example-nginx",
				)

				podOutput, err := utils.Run(cmd)
				gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 0 {
					return fmt.Errorf("expect 0 pods running, but got %d", len(podNames))
				}
				return nil
			}
			gomega.EventuallyWithOffset(1, verifyForceIdlingPods, duration, interval).Should(gomega.Succeed())
			time.Sleep(5 * time.Second)
			// this will trigger the environment to unidle again, but should confirm that the environment is idled
			verifyForceIdling := func() error {
				runCmd := fmt.Sprintf(`curl -s -I http://aergia.%s.nip.io/`, kindIP)
				output, err := utils.RunCommonsCommand(namespace, runCmd)
				fmt.Printf("curl: %s", string(output))
				gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
				err = utils.CheckStringContainsStrings(string(output), []string{"503 Service"})
				if err != nil {
					return err
				}
				return nil
			}
			ginkgo.By("wait for environment to unidle")
			gomega.EventuallyWithOffset(1, verifyForceIdling, duration, interval).Should(gomega.Succeed())
			time.Sleep(5 * time.Second)
			// verify that it eventually unidles
			ginkgo.By("validate environment is unidled")
			gomega.EventuallyWithOffset(1, verifyUnidling, duration, interval).Should(gomega.Succeed())

			ginkgo.By("validating that force unidling works")
			// first force idle the environment
			cmd = exec.Command(
				utils.Kubectl(), "patch", "namespace", "example-nginx",
				"--type", "merge",
				"-p", "{\"metadata\":{\"labels\":{\"idling.amazee.io/force-idled\":\"true\"}}}",
			)
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
			time.Sleep(5 * time.Second)
			// now wait for the environment to idle (no pods)
			gomega.EventuallyWithOffset(1, verifyForceIdlingPods, duration, interval).Should(gomega.Succeed())
			time.Sleep(5 * time.Second)
			// now force unidle
			cmd = exec.Command(
				utils.Kubectl(), "patch", "namespace", "example-nginx",
				"--type", "merge",
				"-p", "{\"metadata\":{\"labels\":{\"idling.amazee.io/unidle\":\"true\"}}}",
			)
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())
			time.Sleep(5 * time.Second)
			// and check that it eventually unidles
			gomega.EventuallyWithOffset(1, verifyUnidling, duration, interval).Should(gomega.Succeed())

			ginkgo.By("validating that unauthenticated metrics requests fail")
			runCmd = `curl -s -k https://aergia-controller-controller-manager-metrics-service.aergia-controller-system.svc.cluster.local:8443/metrics | grep -v "#" | grep "aergia_"`
			_, err = utils.RunCommonsCommand(namespace, runCmd)
			gomega.ExpectWithOffset(2, err).To(gomega.HaveOccurred())

			ginkgo.By("validating that authenticated metrics requests succeed with metrics")
			runCmd = `curl -s -k -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://aergia-controller-controller-manager-metrics-service.aergia-controller-system.svc.cluster.local:8443/metrics | grep -v "#" | grep "aergia_"`
			output, err = utils.RunCommonsCommand(namespace, runCmd)
			gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
			fmt.Printf("metrics: %s", string(output))
			err = utils.CheckStringContainsStrings(string(output), metricLabels)
			gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())

			// verify that default HTTP response code is 404
			ginkgo.By("validating default HTTP response code")
			runCmd = fmt.Sprintf(`curl -s -I http://non-existing-domain.%s.nip.io/`, kindIP)
			output, _ = utils.RunCommonsCommand(namespace, runCmd)
			fmt.Printf("curl: %s", string(output))
			err = utils.CheckStringContainsStrings(string(output), []string{"404 Not Found"})
			gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
			// End tests
		})
	})
})
