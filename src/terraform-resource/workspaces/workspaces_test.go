package workspaces_test

import (
	"errors"
	"terraform-resource/models"
	"terraform-resource/terraform/terraformfakes"
	"terraform-resource/workspaces"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Workspaces", func() {

	Describe("#LatestVersionForEnv", func() {
		var fakeTerraform *terraformfakes.FakeClient

		Context("when the given env does not exist", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.WorkspaceListReturns([]string{}, nil)
				fakeTerraform.InitWithBackendReturns(nil)
			})

			It("returns an empty Version", func() {
				spaces := workspaces.New(fakeTerraform)

				version, err := spaces.LatestVersionForEnv("missing-env")
				Expect(err).To(BeNil())
				Expect(version).To(Equal(models.Version{}))
			})
		})

		Context("when the given env exists", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.WorkspaceListReturns([]string{"some-env"}, nil)
				fakeTerraform.StatePullReturns(map[string]interface{}{"serial": float64(7)}, nil)
			})

			It("returns a Version with the given serial number", func() {
				spaces := workspaces.New(fakeTerraform)

				version, err := spaces.LatestVersionForEnv("some-env")
				Expect(err).To(BeNil())
				Expect(version).To(Equal(models.Version{EnvName: "some-env", Serial: 7}))
			})
		})

		Context("when initializing fails", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.InitWithBackendReturns(errors.New("some-error"))
			})

			It("returns the error", func() {
				spaces := workspaces.New(fakeTerraform)

				_, err := spaces.LatestVersionForEnv("some-env")
				Expect(err).To(MatchError("some-error"))
			})
		})

		Context("when listing workspaces returns an error", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.WorkspaceListReturns(nil, errors.New("some-error"))
			})

			It("returns the error", func() {
				spaces := workspaces.New(fakeTerraform)

				_, err := spaces.LatestVersionForEnv("some-env")
				Expect(err).To(MatchError("some-error"))
			})
		})

		Context("when pulling state returns an error", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.WorkspaceListReturns([]string{"some-env"}, nil)
				fakeTerraform.StatePullReturns(nil, errors.New("some-error"))
			})

			It("returns the error", func() {
				spaces := workspaces.New(fakeTerraform)

				_, err := spaces.LatestVersionForEnv("some-env")
				Expect(err).To(MatchError("some-error"))
			})
		})

		Context("when the serial number is not valid", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.WorkspaceListReturns([]string{"some-env"}, nil)
				fakeTerraform.StatePullReturns(map[string]interface{}{"serial": "nan"}, nil)
			})

			It("returns the error", func() {
				spaces := workspaces.New(fakeTerraform)

				_, err := spaces.LatestVersionForEnv("some-env")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("nan"))
			})
		})
	})
})
