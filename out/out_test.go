package main_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"runtime"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/ljfranklin/terraform-resource/out/models"
	"github.com/ljfranklin/terraform-resource/storage"
	"github.com/ljfranklin/terraform-resource/terraform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var workingDir string

var _ = Describe("Out", func() {

	var (
		storageModel      storage.Model
		stateFileName     string
		subnetCIDR        string
		workingDir        string
		assertOutBehavior func(models.OutRequest, map[string]interface{})
	)

	BeforeEach(func() {
		// TODO: avoid random clashes here
		rand.Seed(time.Now().UnixNano())
		subnetCIDR = fmt.Sprintf("10.0.%d.0/24", rand.Intn(256))

		stateFileName = randomString("out-test")

		storageModel = storage.Model{
			Bucket:          bucket,
			BucketPath:      bucketPath,
			StateFile:       stateFileName,
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
		}

		var err error
		workingDir, err = ioutil.TempDir(os.TempDir(), "terraform-resource-out-test")
		Expect(err).ToNot(HaveOccurred())

		fixturesDir := path.Join(getProjectRoot(), "fixtures")
		err = exec.Command("cp", "-r", fixturesDir, workingDir).Run()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(workingDir)
		awsVerifier.DeleteSubnetWithCIDR(subnetCIDR, vpcID)
		awsVerifier.DeleteObjectFromS3(bucket, path.Join(bucketPath, stateFileName))
	})

	It("creates IaaS resources from a local terraform source", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.Params{
				Terraform: terraform.Model{
					Source: "fixtures/aws/",
					Vars: map[string]interface{}{
						"access_key":  accessKey,
						"secret_key":  secretKey,
						"vpc_id":      vpcID,
						"subnet_cidr": subnetCIDR,
					},
				},
			},
		}
		expectedMetadata := map[string]interface{}{
			"vpc_id":      vpcID,
			"subnet_cidr": subnetCIDR,
			"tag_name":    "terraform-resource-test", // template default
		}

		assertOutBehavior(req, expectedMetadata)
	})

	It("creates IaaS resources from a remote terraform source", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.Params{
				Terraform: terraform.Model{
					// Note: changes to fixture must be pushed before running this test
					Source: "github.com/ljfranklin/terraform-resource//fixtures/aws/",
					Vars: map[string]interface{}{
						"access_key":  accessKey,
						"secret_key":  secretKey,
						"vpc_id":      vpcID,
						"subnet_cidr": subnetCIDR,
					},
				},
			},
		}
		expectedMetadata := map[string]interface{}{
			"vpc_id":      vpcID,
			"subnet_cidr": subnetCIDR,
			"tag_name":    "terraform-resource-test", // template default
		}

		assertOutBehavior(req, expectedMetadata)
	})

	It("creates IaaS resources from `source.terraform.vars`", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
				Terraform: terraform.Model{
					Source: "fixtures/aws/",
					Vars: map[string]interface{}{
						"access_key":  accessKey,
						"secret_key":  secretKey,
						"vpc_id":      vpcID,
						"subnet_cidr": subnetCIDR,
						"tag_name":    "terraform-resource-source-test",
					},
				},
			},
			Params: models.Params{},
		}
		expectedMetadata := map[string]interface{}{
			"vpc_id":      vpcID,
			"subnet_cidr": subnetCIDR,
			"tag_name":    "terraform-resource-source-test",
		}

		assertOutBehavior(req, expectedMetadata)
	})

	It("creates IaaS resources from `source.terraform` and `put.params.terraform`", func() {
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
				Terraform: terraform.Model{
					Source: "fixtures/aws/",
					Vars: map[string]interface{}{
						"access_key": accessKey,
						"secret_key": "bad-secret-key", // will be overridden
					},
				},
			},
			// put params take precedence
			Params: models.Params{
				Terraform: terraform.Model{
					Vars: map[string]interface{}{
						"secret_key":  secretKey,
						"vpc_id":      vpcID,
						"subnet_cidr": subnetCIDR,
						"tag_name":    "terraform-resource-options-test",
					},
				},
			},
		}
		expectedMetadata := map[string]interface{}{
			"vpc_id":      vpcID,
			"subnet_cidr": subnetCIDR,
			"tag_name":    "terraform-resource-options-test",
		}

		assertOutBehavior(req, expectedMetadata)
	})

	Context("when given a yaml file containing variables", func() {
		var varFileName string

		BeforeEach(func() {
			fileParams := map[string]interface{}{
				"vpc_id":   vpcID,
				"tag_name": "terraform-resource-file-test",
			}
			fileContent, err := yaml.Marshal(fileParams)
			Expect(err).ToNot(HaveOccurred())

			varFileName = fmt.Sprintf("%s.tf", randomString("tf-variables"))
			varFilePath := path.Join(workingDir, varFileName)
			varFile, err := os.Create(varFilePath)
			Expect(err).ToNot(HaveOccurred())
			defer varFile.Close()

			_, err = varFile.Write(fileContent)
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates IaaS resources from request vars and file vars", func() {
			req := models.OutRequest{
				Source: models.Source{
					Storage: storageModel,
					Terraform: terraform.Model{
						Source: "fixtures/aws/",
						Vars: map[string]interface{}{
							"access_key": accessKey,
							// will be overridden
							"secret_key": "bad-secret-key",
						},
					},
				},
				// put params overrides source
				Params: models.Params{
					Terraform: terraform.Model{
						Vars: map[string]interface{}{
							"secret_key":  secretKey,
							"subnet_cidr": subnetCIDR,
							// will be overridden
							"tag_name": "terraform-resource-test-original",
						},
						// var file overrides put.params
						VarFile: varFileName,
					},
				},
			}
			expectedMetadata := map[string]interface{}{
				"vpc_id":      vpcID,
				"subnet_cidr": subnetCIDR,
				"tag_name":    "terraform-resource-file-test",
			}

			assertOutBehavior(req, expectedMetadata)
		})
	})

	It("creates IaaS resources with a state file specified under `put.params`", func() {
		storageModel.StateFile = ""
		req := models.OutRequest{
			Source: models.Source{
				Storage: storageModel,
			},
			Params: models.Params{
				StateFile: stateFileName,
				Terraform: terraform.Model{
					Source: "fixtures/aws/",
					Vars: map[string]interface{}{
						"access_key":  accessKey,
						"secret_key":  secretKey,
						"vpc_id":      vpcID,
						"subnet_cidr": subnetCIDR,
					},
				},
			},
		}
		expectedMetadata := map[string]interface{}{
			"vpc_id":      vpcID,
			"subnet_cidr": subnetCIDR,
			"tag_name":    "terraform-resource-test", // template default
		}

		assertOutBehavior(req, expectedMetadata)

		awsVerifier.ExpectS3FileToExist(bucket, path.Join(bucketPath, stateFileName))
	})

	assertOutBehavior = func(outRequest models.OutRequest, expectedMetadata map[string]interface{}) {
		createOutput := models.OutResponse{}
		runOutCommand(outRequest, &createOutput, workingDir)

		Expect(createOutput.Metadata).ToNot(BeEmpty())
		fields := map[string]interface{}{}
		for _, field := range createOutput.Metadata {
			fields[field.Name] = field.Value
		}

		for key, value := range expectedMetadata {
			Expect(fields[key]).To(Equal(value))
		}

		Expect(fields["subnet_id"]).ToNot(BeEmpty())
		subnetID := fields["subnet_id"].(string)
		awsVerifier.ExpectSubnetToExist(subnetID)
		awsVerifier.ExpectSubnetToHaveTags(subnetID, map[string]string{
			"Name": fields["tag_name"].(string),
		})
	}
})

func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(1)
	return path.Join(path.Dir(filename), "..")
}

func randomString(prefix string) string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	Expect(err).ToNot(HaveOccurred())
	return fmt.Sprintf("%s-%x", prefix, b)
}
